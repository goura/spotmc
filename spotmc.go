package spotmc

import (
	"fmt"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/pivotal-golang/archiver/extractor"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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
