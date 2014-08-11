// watch project main.go
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-fsnotify/fsnotify"
)

type Event struct {
	Name string
	Op   int
}

var (
	onUpdateTs   map[string]string
	onUpdateM3u8 map[string]string
	opEvent      [3]chan *Event
)

const (
	root   = "/opt/nginx/stream/nettv"
	low    = root + "/stream_low"
	mid    = root + "/stream_mid"
	hi     = root + "/stream_hi"
	webUrl = "http://128.199.129.74/upstream"
	Update = 0
	Delete = 1
	Low    = 0
	Mid    = 1
	Hi     = 2
)

func webOp(op int, url string, filepath string) {
	var (
		method string
		req    *http.Request
		err    error
	)

	client := &http.Client{}
	switch op {
	case Update:
		method = "PUT"
		file, err := os.Open(filepath)
		if err != nil {
			return
		}
		defer file.Close()
		stat, _ := file.Stat()
		fmt.Println("Transfering", filepath, "to", url, "size", stat.Size())
		req, err = http.NewRequest(method, url, file)
	case Delete:
		method = "DELETE"
		fmt.Println("Deleting", url)
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return
	}
	resp, err := client.Do(req)
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}

func quality(path string) int {
	var quality int
	switch path {
	case "/stream_low":
		quality = Low
	case "/stream_mid":
		quality = Mid
	case "/stream_hi":
		quality = Hi
	}

	return quality
}

func checkUpdatedTs(ev fsnotify.Event) {
	quality := quality(strings.TrimPrefix(path.Dir(ev.Name), root))

	if ev.Op&fsnotify.Remove == fsnotify.Remove {
		opEvent[quality] <- &Event{Name: strings.TrimPrefix(ev.Name, root), Op: Delete}
		return
	} else if ev.Op&fsnotify.Create == fsnotify.Create {
		// no need to track
		return
	}

	dir := path.Dir(ev.Name)

	if curr, ok := onUpdateTs[dir]; !ok {
		// first init
		onUpdateTs[dir] = ev.Name
	} else if curr != ev.Name {
		// there's new updated stream file
		opEvent[quality] <- &Event{Name: strings.TrimPrefix(curr, root), Op: Update}
		onUpdateTs[dir] = ev.Name
	}
}

func checkUpdatedM3u8(ev fsnotify.Event) {
	// more simple than .ts
	quality := quality(strings.TrimPrefix(path.Base(ev.Name), root))
	opEvent[quality] <- &Event{Name: strings.TrimPrefix(ev.Name, root), Op: Update}
}

func showContent(file string) {
	inp, inpErr := os.Open(file)
	if inpErr != nil {
		return
	}
	defer inp.Close()

	reader := bufio.NewReader(inp)
	for {
		inpString, inpSErr := reader.ReadString('\n')
		if inpSErr == io.EOF {
			return
		}
		fmt.Print(inpString)
	}
}

func handleTransfer(quality int) {
	for {
		event := <-opEvent[quality]
		webOp(event.Op, webUrl+event.Name, root+event.Name)
	}
}

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	onUpdateTs = make(map[string]string, 3)
	onUpdateM3u8 = make(map[string]string, 3)
	opEvent[Low] = make(chan *Event, 4)
	opEvent[Mid] = make(chan *Event, 4)
	opEvent[Hi] = make(chan *Event, 4)

	// file events
	go func() {
		for {
			select {
			case ev := <-watcher.Events:
				if strings.HasSuffix(ev.Name, ".ts") {
					checkUpdatedTs(ev)
				} else if strings.HasSuffix(ev.Name, ".m3u8") {
					checkUpdatedM3u8(ev)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(low)
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.Add(mid)
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.Add(hi)
	if err != nil {
		log.Fatal(err)
	}

	// file transfer
	go handleTransfer(Low)
	go handleTransfer(Mid)
	go handleTransfer(Hi)

	// just wait forever
	for {
		time.Sleep(10 * time.Millisecond)
	}
}
