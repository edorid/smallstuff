package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alyu/configparser"
)

var (
	configIni string
	command   string
	port      string
)

func init() {
	const (
		defaultConfig = "./config.ini"
	)
	flag.StringVar(&configIni, "c", defaultConfig, "path to config.ini")
	configIni, _ = filepath.Abs(configIni)
}

func getConfig() *configparser.Configuration {
	cfg, err := configparser.Read(configIni)
	if err != nil {
		fmt.Printf("Cannot read configuration !!")
	}
	return cfg
}

func handleGitHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(w, "Only Gitlab Webhook trigger allowed")
		return
	}
	var data map[string]interface{}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&data); err != nil {
		fmt.Printf("Error processing data")
		return
	}

	repo := data["repository"].(map[string]interface{})
	name := strings.ToLower(repo["name"].(string))

	repoCfg, err := getConfig().Section("repository")
	if err != nil {
		fmt.Printf("Error reading config")
		return
	}

	repoVal := repoCfg.Options()

	if val, ok := repoVal[name]; ok {
		if err := os.Chdir(val); err != nil {
			fmt.Println("Cannot change dir")
			return
		}
		fmt.Printf("Running command %s on %s\n", command, val)
		cmd := exec.Command("/bin/sh", "-c", `"`+command+`"`)
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			fmt.Println("Error running: ", err)
		}
		//fmt.Print(string(out))
	}
}

func main() {
	flag.Parse()

	cfg := getConfig()
	common, err := cfg.Section("common")
	if err != nil {
		log.Fatal("No Global section !!")
	}

	command = common.ValueOf("command")
	port = common.ValueOf("port")

	http.HandleFunc("/", handleGitHook)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
