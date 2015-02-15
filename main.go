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
	log.Info("retrieving game server jar file")
	_, err = smc.getJarFile()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.WithFields(log.Fields{
		"path": smc.serverPath,
	}).Info("game server jar file retrieved")

	// Get the data dir from S3
	log.Info("retrieving data directory")
	_, err = smc.getDataDir()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.WithFields(log.Fields{
		"path": smc.dataDirPath,
	}).Info("data directory archive file retrieved")

	// Run game server
	log.Printf("starting the game server")
	cmd, err := smc.startServer()
	if err != nil {
		err = fmt.Errorf("game server did not start: %s", err)
	}

	// Spawn watch proc which waits for the game server to end
	go func() {
		err = cmd.Wait()
		log.Info("game server process exited")
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
			log.Info("killing the game server")
			cmd.Process.Kill()
		}
		if msg == msgShutdownCluster {
			log.Info("shutting down the cluster")
			err := smc.shutdownCluster()
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Fatal("cluster shutdown failed!")
			}
			go func() {
				smc.msgs <- msgInstanceTerminating
			}()
		}
		if msg == msgGameServerDown {
			// If the game server ends, the instance dies

			// Save data to S3
			log.Info("saving data to S3 started")
			err := smc.putDataDir()
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Fatal("saving data to S3 failed")
			} else {
				log.Info("saving data to S3 done")
			}

			// Kill instance
			smc.killInstance()

			// Escape the loop
			break
		}
	}
}
