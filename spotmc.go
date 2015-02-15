package spotmc

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/pivotal-golang/archiver/extractor"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var AWS_RETRY = 3
var JAR_PATH_DIR = ""
var JAR_PATH_PREFIX = "mcjar"
var DATA_PATH_DIR = ""
var DATA_PATH_PREFIX = "mcdata"
var TERMINATION_TIME_URL = "http://169.254.169.254/latest/meta-data/spot/termination-time"

const (
	msgInstanceTerminating = iota
	msgShutdownCluster
	msgGameServerDown
)

// Defaults
var DEFAULT_KILL_INSTANCE_MODE = "false"
var DEFAULT_SHUTDOWN_CMD = "/sbin/shutdown -h now"
var DEFAULT_REGION = "ap-northeast-1"
var DEFAULT_MAX_UPTIME = 43200
var DEFAULT_MAX_IDLE_TIME = 14400
var DEFAULT_IDLE_WATCH_PATH = "world/playerdata"
var DEFAULT_IDLE_WATCH_GRACE_TIME = 600

type SpotMC struct {
	JarFileURL         string
	EULAFileURL        string
	DataFileURL        string
	JavaPath           string
	JavaArgs           string
	serverPath         string
	dataDirPath        string
	ddnsURL            string
	killInstanceMode   string
	maxIdleTime        int
	maxUptime          int
	shutdownCommand    string
	idleWatchGraceTime int
	idleWatchPath      string
	autoScalingGroup   string
	msgs               chan int
}

func NewSpotMC() (*SpotMC, error) {
	// Check mandatory environment variables
	for _, k := range []string{
		"SPOTMC_SERVER_JAR_URL",
		"SPOTMC_SERVER_EULA_URL",
		"SPOTMC_DATA_URL",
		"SPOTMC_JAVA_PATH",
	} {
		s := os.Getenv(k)
		if s == "" {
			return nil, fmt.Errorf("set valid env vars")
		}
	}

	// Kill instance mode
	// "shutdown" or "false"
	killInstanceMode := DEFAULT_KILL_INSTANCE_MODE
	s := os.Getenv("SPOTMC_KILL_INSTANCE_MODE")
	if s != "" {
		killInstanceMode = s
	}
	shutdownCommand := DEFAULT_SHUTDOWN_CMD
	s = os.Getenv("SPOTMC_SHUTDOWN_CMD")
	if s != "" {
		shutdownCommand = s
	}

	// Max uptime and max idle time
	maxIdleTime := DEFAULT_MAX_IDLE_TIME
	s = os.Getenv("SPOTMC_MAX_IDLE_TIME")
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

	// Idle watch grace time
	idleWatchGraceTime := DEFAULT_IDLE_WATCH_GRACE_TIME
	s = os.Getenv("SPOTMC_IDLE_WATCH_GRACE_TIME")
	if s != "" {
		i, err := strconv.Atoi(s)
		if err == nil {
			idleWatchGraceTime = i
		}
	}

	// DDNS Update URL
	ddnsURL := os.Getenv("SPOTMC_DDNS_UPDATE_URL")

	// AutoScaling Group
	autoScalingGroup := os.Getenv("SPOTMC_AUTOSCALING_GROUP")

	smc := &SpotMC{
		JarFileURL:         os.Getenv("SPOTMC_SERVER_JAR_URL"),
		EULAFileURL:        os.Getenv("SPOTMC_SERVER_EULA_URL"),
		DataFileURL:        os.Getenv("SPOTMC_DATA_URL"),
		JavaPath:           os.Getenv("SPOTMC_JAVA_PATH"),
		JavaArgs:           os.Getenv("SPOTMC_JAVA_ARGS"),
		ddnsURL:            ddnsURL,
		killInstanceMode:   killInstanceMode,
		maxIdleTime:        maxIdleTime,
		maxUptime:          maxUptime,
		shutdownCommand:    shutdownCommand,
		idleWatchGraceTime: idleWatchGraceTime,
		idleWatchPath:      idleWatchPath,
		autoScalingGroup:   autoScalingGroup,
	}

	return smc, nil
}

func (smc *SpotMC) getJarFile() (serverPath string, err error) {
	dir, err := ioutil.TempDir(JAR_PATH_DIR, JAR_PATH_PREFIX)
	if err != nil {
		return "", err
	}
	serverPath = dir + "/server.jar"

	err = S3Get(smc.JarFileURL, serverPath)
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

	err = S3Get(smc.DataFileURL, tgzFile.Name())
	if err != nil {
		// Maybe the first time, it's ok.
		// Populate the data dir with user-provided eula.txt
		log.WithFields(log.Fields{"url": smc.EULAFileURL}).Info("downloading EULA file")
		eulaFilePath := dataDirPath + "/eula.txt"
		err2 := S3Get(smc.EULAFileURL, eulaFilePath)
		if err2 != nil {
			log.WithFields(log.Fields{"err": err2}).Error("downloading EULA file failed")
			return "", err2
		}
		log.WithFields(log.Fields{"path": eulaFilePath}).Info("EULA file")
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
	err = S3Put(smc.DataFileURL, tgzFile.Name())
	return err
}

func (smc *SpotMC) updateDDNS() {
	if smc.ddnsURL != "" {
		log.Info("Issuing DDNS query")
		resp, err := http.Get(smc.ddnsURL)
		resp.Body.Close()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Warn("DDNS update query failed")
		} else {
			log.WithFields(log.Fields{"status": resp.Status}).Info("DDNS update query")
		}
	}
}

func (smc *SpotMC) startServer() (exec.Cmd, error) {
	args := []string{smc.JavaPath}
	if smc.JavaArgs != "" {
		extraArgs := strings.Split(smc.JavaArgs, " ")
		args = append(args, extraArgs...)
	}
	args = append(args, "-jar", smc.serverPath, "nogui")

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
		Dir:  smc.dataDirPath,
		//Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err := cmd.Start()

	log.WithFields(log.Fields{
		"cmd": args[0], "args": args, "len": len(args), "err": err,
	}).Info("Executed command")

	return cmd, err
}

func (smc *SpotMC) killInstance() error {
	log.WithFields(log.Fields{
		"killInstanceMode": smc.killInstanceMode,
	}).Info("killInstance invoked")

	if smc.killInstanceMode == "false" {
		// False mode. Don't do anything
		return nil
	}

	if smc.killInstanceMode == "shutdown" {
		cmdArgs := strings.Split(smc.shutdownCommand, " ")
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		err := cmd.Run()
		return err
	}

	return nil
}

func (smc *SpotMC) shutdownCluster() error {
	log.WithFields(log.Fields{
		"autoScalingGroup": smc.autoScalingGroup,
	}).Info("ShutDownCluster invoked")

	var err error
	if smc.autoScalingGroup != "" {
		log.Info("setting cluster capacity to 0")
		for i := 0; i < AWS_RETRY; i++ {
			err = SetDesiredCapacity(smc.autoScalingGroup, 0)
			if err == nil {
				log.Info("SetDesiredCapacity succeeded")
				break
			}
			log.WithFields(log.Fields{"err": err}).Error("failed to set cluster capacity, retrying")
		}
	}
	if err != nil {
		log.Fatal("failed to set cluster capacity")
	}
	return err
}

// uptimeWatcher() shutdowns the *cluster* when
// the process uptime exceeds the predefined limit (smc.maxUptime).
func (smc *SpotMC) uptimeWatcher() {
	logFields := log.Fields{"maxUptime": smc.maxUptime}
	log.WithFields(logFields).Info("uptimeWatcher starting")

	d := time.Duration(smc.maxUptime) * time.Second
	time.Sleep(d)

	log.WithFields(logFields).Info("uptime exceeded limit, shutdown the cluster")
	smc.msgs <- msgShutdownCluster
}

// idleWatcher() shutdowns the *cluster* when
// there's a long idle time (smc.maxIdleTime).
func (smc *SpotMC) idleWatcher() {
	grace := time.Duration(smc.idleWatchGraceTime) * time.Second
	log.Infof("idle watcher starts after %.2f mins", grace.Minutes())
	time.Sleep(grace) // Wait for a grace period

	fullPath := smc.dataDirPath + "/" + smc.idleWatchPath
	d := time.Duration(smc.maxIdleTime) * time.Second

	log.WithFields(log.Fields{"idleWatchPath": fullPath, "maxIdleTime": smc.maxIdleTime}).Info("idle watcher starting")

	for true {
		time.Sleep(d / 12)
		fi, err := os.Stat(fullPath)
		if err != nil {
			log.Printf("os.Stat failed(%s): %s", fullPath, err)
			log.WithFields(log.Fields{"idleWatchPath": fullPath, "err": err}).Error("os.Stat failed")
			continue
		}
		mtime := fi.ModTime()
		log.Infof("time.Since(mtime): %.2f minutes (%s)",
			time.Since(mtime).Minutes(), fullPath)
		if time.Since(mtime) > d {
			log.Infof("idle time exceeded limit, shutdown the cluster")
			break
		}
	}
	smc.msgs <- msgShutdownCluster
}

// terminationNotificationWatcher() accesses EC2 meta-data and
// watches spot instance shutdown notification.
// It sends a message to kill the game server before
// the actual shutdown process starts
func (smc *SpotMC) terminationNotificationWatcher() {
	d := time.Duration(10) * time.Second
	for {
		time.Sleep(d)
		resp, err := http.Get(TERMINATION_TIME_URL)
		resp.Body.Close()
		log.WithFields(log.Fields{
			"url":    TERMINATION_TIME_URL,
			"status": resp.StatusCode,
			"err":    err,
		}).Debug("termination check result")
		// 404 means termination is not scheduled,
		// 200 means termination is scheduled
		if resp.StatusCode == 200 {
			log.WithFields(log.Fields{
				"status": resp.StatusCode,
			}).Info("termination schedule detected, kill this instance beforehand")
			smc.msgs <- msgInstanceTerminating
			break
		}
	}
}
