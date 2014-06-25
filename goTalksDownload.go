package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"code.google.com/p/go.net/html"
	
	"github.com/brunetto/goutils/debug"
	"github.com/brunetto/goutils"
)

// Download talks (pdf files) from a webpage

func main () () {
	defer debug.TimeMe(time.Now())
	
	var (
		nProcs int = 4 
		u *url.URL
		resp *http.Response
		err error
		tokenizer *html.Tokenizer
		token html.Token
		talkChan = make(chan string, 50)
		done = make(chan struct{}, nProcs)
		originalUrlPath string
	)
	
	if len(os.Args) < 2 {
		log.Fatal("Provide a page to scan for talks")
	}
	
	log.Println("Start ", nProcs, " downloader goroutines")
	for idx:=0; idx<nProcs; idx++ {
		go downloadTalk(talkChan, done)
	}
	
	// Check if http present, in case, add it
	u, err = url.Parse(os.Args[1])
	if u.Scheme == "" {u.Scheme="http"}
	
	log.Println("Scanning: ", u.String())
		
	resp, err = http.Get(u.String())
	if err != nil {
		log.Fatal("Can't get page: ", err)
	}
	defer resp.Body.Close()
	
	// Create tokenizer for the html page
	tokenizer = html.NewTokenizer(resp.Body)
	
	// Save original path 
	originalUrlPath = filepath.Dir(u.Path)
	log.Println("path ", originalUrlPath)
	
	log.Println("Start parsing")
	for {
		// Walk the page tokens
		if tokenizer.Next() == html.ErrorToken {
			// Returning io.EOF indicates success.
			err = tokenizer.Err()
			if err.Error() != "EOF" {
				fmt.Println(err.Error())
			}
			break
		}
		// Take the token
		token = tokenizer.Token()
		// Check if a pdf link
		if strings.Contains(token.String(), "href") && strings.Contains(token.String(), ".pdf") {
			// Compose the pdf url
			u.Path = filepath.Join(originalUrlPath, token.Attr[0].Val)
			// Send url to downloader
			talkChan <- u.String()
			fmt.Println("Sent ", u.String())
		}
	}
	log.Println("Done parsing")
	
	// Close channel, if you forget it, goroutine will wait forever
	close(talkChan)
	
	// Wait goroutine to finish
	for idx:=0; idx<nProcs; idx++ {
		<- done
	}
	log.Println("Done downloading")
}

func downloadTalk(talkChan chan string, done chan struct{}) () {
	var (
		response *http.Response
		err error
		talkUrl string
		outFile *os.File
	)
	
	// Receive urls
	for talkUrl = range talkChan {
		log.Println("Downloading ", filepath.Base(talkUrl))
		
		// Check if file exists, in case, skip
		if goutils.Exists(filepath.Base(talkUrl)) {
			log.Println("File already exists, skip and go to the next!")
			continue
		}
		
		// Create local file to copy to
		if outFile, err = os.Create(filepath.Base(talkUrl)); err != nil {
			log.Println("Error while creating ", talkUrl, ": ", err)
        }
        defer outFile.Close()
		
		// Download data
		if response, err = http.Get(talkUrl); err != nil {
            log.Println("Error while downloading ", talkUrl, ": ", err)
        }
        defer response.Body.Close()
		
		// Copy data to file
		if _, err = io.Copy(outFile, response.Body); err != nil {
            log.Println("Error while copying ", talkUrl, ": ", err)
			log.Println("Removing broken file...")
			if err = os.Remove(filepath.Base(talkUrl)); err != nil {
				log.Fatal("Error while removing broken file: ", err)
			}
        }
	}
	// Send "done" signal
	done <- struct{}{}
}

