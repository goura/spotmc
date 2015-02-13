package spotmc

import (
	"fmt"
	"log"
	"os"
	"time"
)

var JAR_PATH_DIR = ""
var JAR_PATH_PREFIX = "mcjar"
var DATA_PATH_DIR = ""
var DATA_PATH_PREFIX = "mcdata"
var AWS_RETRY = 3

// Defaults
var DEFAULT_REGION = "ap-northeast-1"
var DEFAULT_MAX_UPTIME = 43200
var DEFAULT_MAX_IDLE_TIME = 14400
var DEFAULT_IDLE_WATCH_PATH = "world/playerdata"

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
		grace := time.Minute * 10
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
	}()

	// Spawn yet yet another watch proc
	// This process waits for the game server to end
	go func() {
		err = cmd.Wait()
		log.Printf("game server process exited")
		msgs <- "server_down"
	}()

	loop := true
	for loop {
		select {
		case msg := <-msgs:
			if msg == "server_down" {
				// If the game server ends, the instance dies
				loop = false
				smc.KillInstance()
				break
			}
			if msg == "shutdown_cluster" {
				if smc.autoScalingGroup != "" {
					log.Printf("setting cluster capacity to 0")
					for i := 0; i < AWS_RETRY; i++ {
						err := setDesiredCapacity(smc.autoScalingGroup, 0)
						if err == nil {
							break
						}
					}
					log.Fatal("setDesiredCapacity Failed!")
				}
				log.Printf("killing the game server")
				cmd.Process.Kill()
			}
		}
	}
}
