package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"
)

var cache = make(map[string]string)
var rss Rss

type Rss struct {
	Channel *Channel `xml:"channel"`
}

type Channel struct {
	ItemList []Item `xml:"item"`
}

type Item struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

func fetchXML() (Rss, error) {

	resp, err := http.Get(fmt.Sprintf("%s/feed", cfg.BaseUrl))

	if err != nil {
		log.Printf("fetchXML: failed to fetch rss feed %s\n", err)
		return Rss{}, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Printf("fetchXML: failure in reading body of the response %s\n", err)
		return Rss{}, err
	}
	var rss Rss

	if err := xml.Unmarshal(body, &rss); err != nil {
		log.Printf("fetchXML: failure in parsing XML body %s\n", err)
		return Rss{}, err
	}
	return rss, nil
}
func getXML() {

	var updates = make(chan Rss)
	var ticker = time.Tick(10 * time.Second)

	fetch := func() {
		log.Println("getXML: performing an rss fetch")
		rssFeed, err := fetchXML()
		if err != nil {
			log.Printf("getXML: fetch failed: %s", err)
			return
		}
		updates <- rssFeed
	}

	fetch()

	for {
		select {
		case <-ticker:
			fetch()
		case rss = <-updates:

			sort.Slice(rss.Channel.ItemList, func(i, j int) bool {
				timeI, err1 := time.Parse(time.RFC1123, rss.Channel.ItemList[i].PubDate)
				timeJ, err2 := time.Parse(time.RFC1123, rss.Channel.ItemList[j].PubDate)

				if err1 != nil || err2 != nil {
					return false
				}
				return timeJ.After(timeI)

			})
			fmt.Println(rss.Channel.ItemList)
		}
	}

}
