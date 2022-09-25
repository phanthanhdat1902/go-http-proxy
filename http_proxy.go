package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/kardianos/service"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// default coonfig
const serviceName = "ProxyService"
const serviceDescription = "ProxyService"
const timeout = 10
const defaultPort = "8081"
const defaultCycleTime = 2 // second
const blockAllChar = '*'
const blockAllString = "*.*"

var m sync.Mutex
var blackList []string

type Config struct {
	port      string `json:"port"`
	cycleTime int    `json:"cycleTime"` //second
}

type program struct{}

func (p program) Start(s service.Service) error {
	fmt.Println(s.String() + " started")
	go p.run()
	return nil
}

func (p program) Stop(s service.Service) error {
	fmt.Println(s.String() + " stopped")
	return nil
}

func (p program) run() {
	//create log file
	currentFolder := ""
	for index, element := range os.Args {
		if index != 0 {
			if index == 1 {
				currentFolder = element
			} else {
				currentFolder = currentFolder + " " + element
			}
		}
		// index is the index where we are
		// element is the element from someSlice for where we are
	}
	//currentFolder = strings.Replace(currentFolder, " ", "%20", -1)
	// read config and set proxy port
	configFile, err := os.Open(currentFolder + "/config.json")
	log.Println(currentFolder)
	port := defaultPort
	cycleTime := defaultCycleTime
	if err != nil {
		fmt.Println("config file not right struct, proxy use default port (8081)")
	} else {
		configFileByte, _ := ioutil.ReadAll(configFile)
		var config map[string]string
		json.Unmarshal([]byte(configFileByte), &config)
		// check format
		if _, ok := config["port"]; ok {
			port = config["port"]
		} else {
			fmt.Println("config file not right struct, proxy use default port (8081)")
		}
		if _, ok := config["cycleTime"]; ok {
			cycleTime, err = strconv.Atoi(config["cycleTime"])
			if err != nil {
				fmt.Println("cycle time is not integer value, proxy use default cycle time (2 second)")
				cycleTime = defaultCycleTime
				fmt.Println(cycleTime)
			}
		} else {
			fmt.Println("config file not right struct, proxy use default port (8081)")
			port = defaultPort
		}
	}
	log.Println(port, cycleTime)
	//run update
	go updateBlackList(currentFolder, cycleTime)
	fmt.Println("Proxy running, port " + port)
	// init server
	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//fmt.Println(r.Method)
			//fmt.Println(r.Host)
			//var err error
			//blackList, err = readLines(currentFolder + "/blacklist.txt")
			//if err != nil {
			//	log.Fatalf("readLines: %s", err)
			//}
			//fmt.Println(r.Host)
			logFile := time.Now().Format("02-01-2006")
			m.Lock()
			f, err := os.OpenFile(currentFolder+"/"+logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Fatalf("error opening file: %v", err)
			}
			log.SetOutput(f)
			log.Println(r.Host)
			f.Close()
			m.Unlock()
			if !filter(blackList, r.Host) {
				handleConnect(w, r)
			}
		}),
	}
	// listen
	server.ListenAndServe()
}

// handle Connect Package

func handleConnect(w http.ResponseWriter, r *http.Request) {
	// establishes network connections with timeout
	destConn, err := net.DialTimeout("tcp", r.Host, timeout*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// set header
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking Interface not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	//copy data and send
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

// copy data body

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

//read file

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func updateBlackList(currentFolder string, cycleTime int) []string {
	for {
		//update black list
		var err error
		blackList, err = readLines(currentFolder + "/blacklist.txt")
		if err != nil {
			log.Fatalf("readLines: %s", err)
		}
		time.Sleep(time.Duration(cycleTime) * time.Second)
	}
}

func filter(blackList []string, host string) bool {
	domain := strings.Split(host, ":")[0]
	var regexBlackWeb = ""
	for _, blackWeb := range blackList {
		if blackWeb != "" {
			if blackWeb == blockAllString {
				return true
			} else if strings.Contains(blackWeb, string(blockAllChar)) {
				//regexBlackWeb = strings.Replace(blackWeb, "*", ".*", -1)
				if blackWeb[0] == blockAllChar {
					regexBlackWeb = blackWeb[1:] + "$"
				}
				if blackWeb[len(blackWeb)-1] == blockAllChar {
					regexBlackWeb = blackWeb[:len(blackWeb)-1] + "^"
				}
				isMatch, err := regexp.MatchString(regexBlackWeb, domain)
				if err != nil {
					fmt.Println(err)
				}
				return isMatch
			}
			if strings.Contains(host, blackWeb) {
				return true
			}
			regexBlackWeb = ""
		}
	}
	return false
}

func main() {
	serviceConfig := &service.Config{
		Name:        serviceName,
		DisplayName: serviceName,
		Description: serviceDescription,
	}
	prg := &program{}
	s, err := service.New(prg, serviceConfig)
	if err != nil {
		fmt.Println("Cannot create the service: " + err.Error())
	}
	err = s.Run()
	if err != nil {
		fmt.Println("Cannot start the service: " + err.Error())
	}
}
