package spotmc

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var TERMINATION_TIME_URL = "http://169.254.169.254/latest/meta-data/spot/termination-time"

func Main() {
	// Initialize
	smc, err := NewSpotMC()
	if err != nil {
		panic(err)
	}

	// Update DDNS
	smc.updateDDNS()

	// Get game server jar file
	log.Printf("retrieving game server jar file")
	_, err = smc.getJarFile()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("game server path: %s", smc.serverPath)

	// Get the data dir from S3
	log.Printf("retrieving data directory")
	_, err = smc.getDataDir()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("data directory: %s", smc.dataDirPath)

	// Run game server
	log.Printf("starting the game server")
	cmd, err := smc.StartServer()
	if err != nil {
		err = fmt.Errorf("game server did not start: %s", err)
	}

	msgs := make(chan string)

	// Spawn watch proc
	// This process shutdowns the *cluster* when there's a long idle time.
	go func() {
		grace := time.Duration(smc.idleWatchGraceTime) * time.Second
		log.Printf("idle watcher starts after %.2f mins", grace.Minutes())
		time.Sleep(grace) // Wait for a grace period
		log.Printf("idle watcher starting")

		fullPath := smc.dataDirPath + "/" + smc.idleWatchPath
		d := time.Duration(smc.maxIdleTime) * time.Second
		for true {
			time.Sleep(d / 12)
			fi, err := os.Stat(fullPath)
			if err != nil {
				log.Printf("os.Stat failed(%s): %s", fullPath, err)
				continue
			}
			mtime := fi.ModTime()
			log.Printf("time.Since(mtime): %.2f minutes (%s)",
				time.Since(mtime).Minutes(), fullPath)
			if time.Since(mtime) > d {
				log.Printf("idle time exceeded limit, shutdown the cluster")
				break
			}
		}
		msgs <- "shutdown_cluster"
	}()

	// Spawn another watch proc
	// This process shutdowns the *cluster* when the process uptime exceeds
	// the predefined limit.
	go func() {
		d := time.Duration(smc.maxUptime) * time.Second
		time.Sleep(d)
		log.Printf("uptime exceeded limit, shutdown the cluster")
		msgs <- "shutdown_cluster"
	}()

	// Spawn yet another watch proc
	// This process watches spot instance shutdown notification
	// and kills the game server before the actual shutdown process starts
	go func() {
		d := time.Duration(10) * time.Second
		for {
			time.Sleep(d)
			resp, err := http.Get(TERMINATION_TIME_URL)
			resp.Body.Close()
			log.Printf("termination time url: %s (err:%s)", resp.Status, err)
			// 404 means termination is not scheduled,
			// 200 means termination is scheduled
			if resp.StatusCode != 404 {
				msgs <- "kill_game_server"
				break
			}
		}
	}()

	// Spawn yet yet another watch proc
	// This process waits for the game server to end
	go func() {
		err = cmd.Wait()
		log.Printf("game server process exited")
		msgs <- "game_server_down"
	}()

	// Spawn yet yet yet another proc to handle SIGTERM
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM)
	go func() {
		<-sigchan
		msgs <- "kill_game_server"
	}()

	for {
		msg := <-msgs
		if msg == "kill_game_server" {
			log.Printf("killing the game server")
			cmd.Process.Kill()
		}
		if msg == "shutdown_cluster" {
			log.Printf("shutting down the cluster")
			err := smc.ShutdownCluster()
			if err != nil {
				log.Fatal("cluster shutdown failed!: %s", err)
			}
			go func() {
				msgs <- "kill_game_server"
			}()
		}
		if msg == "game_server_down" {
			// If the game server ends, the instance dies

			// Save data to S3
			log.Print("saving data to S3 started")
			err := smc.putDataDir()
			if err != nil {
				log.Printf("saving data to S3 failed: %s", err)
			} else {
				log.Printf("saving data to S3 done")
			}

			// Kill instance
			smc.KillInstance()

			// Escape the loop
			break
		}
	}
}
