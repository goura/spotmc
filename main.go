package spotmc

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func Main() {
	// Initialize
	smc, err := NewSpotMC()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
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
	cmd, err := smc.startServer()
	if err != nil {
		err = fmt.Errorf("game server did not start: %s", err)
	}

	// Spawn watch proc which waits for the game server to end
	go func() {
		err = cmd.Wait()
		log.Printf("game server process exited")
		smc.msgs <- msgGameServerDown
	}()

	// Spawn a SIGTERM handler
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM)
	go func() {
		<-sigchan
		smc.msgs <- msgInstanceTerminating
	}()

	// Spawn other watchers
	go smc.idleWatcher()
	go smc.uptimeWatcher()
	go smc.terminationNotificationWatcher()

	// Start the main loop
	for {
		msg := <-smc.msgs
		if msg == msgInstanceTerminating {
			log.Printf("killing the game server")
			cmd.Process.Kill()
		}
		if msg == msgShutdownCluster {
			log.Printf("shutting down the cluster")
			err := smc.shutdownCluster()
			if err != nil {
				log.Fatal("cluster shutdown failed!: %s", err)
			}
			go func() {
				smc.msgs <- msgInstanceTerminating
			}()
		}
		if msg == msgGameServerDown {
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
			smc.killInstance()

			// Escape the loop
			break
		}
	}
}
