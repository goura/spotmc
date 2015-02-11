package spotmc

import (
	"fmt"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/pivotal-golang/archiver/extractor"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

type SpotMC struct {
	JarFileURL       string
	EULAFileURL      string
	DataFileURL      string
	JavaPath         string
	JavaArgs         string
	serverPath       string
	dataDirPath      string
	ddnsURL          string
	maxIdleTime      int
	maxUptime        int
	idleWatchPath    string
	autoScalingGroup string
}

func NewSpotMC() (*SpotMC, error) {
	// Check environment variables
	for _, k := range []string{"SPOTMC_SERVER_JAR_URL", "SPOTMC_SERVER_EULA_URL", "SPOTMC_DATA_URL", "SPOTMC_JAVA_PATH"} {
		s := os.Getenv(k)
		if s == "" {
			return nil, fmt.Errorf("set valid env vars")
		}
	}

	// Uptime and IdleTime
	maxIdleTime := DEFAULT_MAX_IDLE_TIME
	s := os.Getenv("SPOTMC_MAX_IDLE_TIME")
	if s != "" {
		i, err := strconv.Atoi(s)
		if err == nil {
			maxIdleTime = i
		}
	}

	maxUptime := DEFAULT_MAX_UPTIME
	s = os.Getenv("SPOTMC_MAX_UPTIME")
	if s != "" {
		i, err := strconv.Atoi(s)
		if err == nil {
			maxUptime = i
		}
	}

	// Idle watch path
	idleWatchPath := DEFAULT_IDLE_WATCH_PATH
	s = os.Getenv("SPOTMC_IDLE_WATCH_PATH")
	if s != "" {
		idleWatchPath = s
	}

	// DDNS Update URL
	ddnsURL := os.Getenv("SPOTMC_DDNS_UPDATE_URL")

	// AutoScaling Group
	autoScalingGroup := os.Getenv("SPOTMC_AUTOSCALING_GROUP")

	smc := &SpotMC{
		JarFileURL:       os.Getenv("SPOTMC_SERVER_JAR_URL"),
		EULAFileURL:      os.Getenv("SPOTMC_SERVER_EULA_URL"),
		DataFileURL:      os.Getenv("SPOTMC_DATA_URL"),
		JavaPath:         os.Getenv("SPOTMC_JAVA_PATH"),
		JavaArgs:         os.Getenv("SPOTMC_JAVA_ARGS"),
		ddnsURL:          ddnsURL,
		maxIdleTime:      maxIdleTime,
		maxUptime:        maxUptime,
		idleWatchPath:    idleWatchPath,
		autoScalingGroup: autoScalingGroup,
	}

	return smc, nil
}

func (smc *SpotMC) getJarFile() (serverPath string, err error) {
	dir, err := ioutil.TempDir(JAR_PATH_DIR, JAR_PATH_PREFIX)
	if err != nil {
		return "", err
	}
	serverPath = dir + "/server.jar"

	err = s3Get(smc.JarFileURL, serverPath)
	if err != nil {
		return "", err
	}

	smc.serverPath = serverPath
	return serverPath, nil
}

func (smc *SpotMC) getDataDir() (dataDirPath string, err error) {
	// Create the data dir
	dataDirPath, err = ioutil.TempDir(DATA_PATH_DIR, DATA_PATH_PREFIX)
	if err != nil {
		return "", err
	}

	// Get tgz file and uncompress it to the data dir
	tgzFile, err := ioutil.TempFile("", "")
	if err != nil {
		return "", err
	}
	tgzFile.Close()

	err = s3Get(smc.DataFileURL, tgzFile.Name())
	if err != nil {
		// Maybe the first time, it's ok.
		// Populate the data dir with user-provided eula.txt
		log.Printf("downloading EULA file: %s", smc.EULAFileURL)
		eulaFilePath := dataDirPath + "/eula.txt"
		err2 := s3Get(smc.EULAFileURL, eulaFilePath)
		if err2 != nil {
			return "", err2
		}
		log.Printf("EULA file path: %s", eulaFilePath)
	} else {
		tgz := extractor.NewTgz()
		err = tgz.Extract(tgzFile.Name(), dataDirPath)
	}

	smc.dataDirPath = dataDirPath
	return dataDirPath, nil
}

func (smc *SpotMC) putDataDir() error {
	// Create a tempfile
	tgzFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	tgzFile.Close()
	defer func() { os.Remove(tgzFile.Name()) }()

	// Compress dir to tgz
	tgz := compressor.NewTgz()
	err = tgz.Compress(smc.dataDirPath+"/", tgzFile.Name())
	if err != nil {
		return err
	}

	// Put tgz to S3
	err = s3Put(smc.DataFileURL, tgzFile.Name())
	return err
}

func (smc *SpotMC) updateDDNS() {
	if smc.ddnsURL != "" {
		log.Printf("issuing DDNS update query")
		resp, err := http.Get(smc.ddnsURL)
		if err != nil {
			log.Printf("DDNS update query failed: %s", err)
		} else {
			log.Printf("DDNS update query result: %s", resp.Status)
		}
	}
}

func (smc *SpotMC) StartServer() (exec.Cmd, error) {
	args := []string{smc.JavaPath}
	if smc.JavaArgs != "" {
		extraArgs := strings.Split(smc.JavaArgs, " ")
		args = append(args, extraArgs...)
	}
	args = append(args, "-jar", smc.serverPath, "nogui")
	log.Printf("Command to execute:%s args:%s length:%d", args[0], args, len(args))

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
		Dir:  smc.dataDirPath,
		//Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err := cmd.Start()
	return cmd, err
}

func (smc *SpotMC) KillInstance() {
	log.Printf("KillInstance invoked")
	log.Print("saving data to S3 started")
	smc.putDataDir()
	log.Print("saving data to S3 done")
	// TODO: kill the instance
}

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
			var st unix.Stat_t
			err := unix.Stat(fullPath, &st)
			if err != nil {
				log.Printf("syscall.Stat failed(%s): %s", fullPath, err)
				continue
			}
			mtime := time.Unix(st.Mtimespec.Sec, 0)
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
