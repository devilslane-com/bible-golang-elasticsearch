package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/fatih/color"
)

type SearchResult struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source struct {
				Book    string `json:"abbrev"`
				Chapter int    `json:"chapter"`
				Verse   int    `json:"verse"`
				Text    string `json:"en"`
			} `json:"_source"`
			Highlight map[string][]string `json:"highlight"`
			Score     float64             `json:"_score"`
		} `json:"hits"`
	} `json:"hits"`
}

func main() {
	var (
		esHost     string
		esIndex    string
		esUsername string
		esPassword string
		searchText string
		maxResults int
	)

	flag.StringVar(&esHost, "host", "https://localhost:9200", "Elasticsearch host")
	flag.StringVar(&esIndex, "index", "bible", "Elasticsearch index name")
	flag.StringVar(&esUsername, "username", "", "Elasticsearch username")
	flag.StringVar(&esPassword, "password", "", "Elasticsearch password")
	flag.StringVar(&searchText, "text", "", "Text to search for")
	flag.IntVar(&maxResults, "max", 25, "Maximum number of search results to return")
	flag.Parse()

	// Override command-line args with environment variables if they exist
	if host := os.Getenv("ES_HOST"); host != "" {
		esHost = host
	}
	if index := os.Getenv("ES_INDEX"); index != "" {
		esIndex = index
	}
	if username := os.Getenv("ES_USERNAME"); username != "" {
		esUsername = username
	}
	if password := os.Getenv("ES_PASSWORD"); password != "" {
		esPassword = password
	}
	if text := os.Getenv("SEARCH_TEXT"); text != "" {
		searchText = text
	}
	if envMaxResults := os.Getenv("MAX_RESULTS"); envMaxResults != "" {
		var err error
		maxResults, err = strconv.Atoi(envMaxResults)
		if err != nil {
			log.Fatalf("Error parsing MAX_RESULTS environment variable: %v", err)
		}
	}

	// Check if required configurations are missing
	if esHost == "" || esIndex == "" || esUsername == "" || esPassword == "" || searchText == "" {
		fmt.Println("Missing required configurations. Please set via environment variables or command-line arguments.")
		os.Exit(1)
	}

	searchTermRegex, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(searchText))
	if err != nil {
		fmt.Printf("Error compiling regex: %v\n", err)
		return
	}

	cfg := elasticsearch.Config{
		Addresses: []string{esHost},
		Username:  esUsername,
		Password:  esPassword,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"match": map[string]interface{}{
							"en": map[string]interface{}{
								"query":     searchText,
								"fuzziness": "AUTO",
							},
						},
					},
					map[string]interface{}{
						"match_phrase": map[string]interface{}{
							"en": searchText,
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"en": map[string]interface{}{},
			},
		},
		"size": maxResults,
	}

	var buf strings.Builder
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}

	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(esIndex),
		es.Search.WithBody(strings.NewReader(buf.String())),
		es.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	var r SearchResult
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}

	if r.Hits.Total.Value == 0 {
		fmt.Println("No results found.")
		return
	}

	blue := color.New(color.FgBlue).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	for _, hit := range r.Hits.Hits {
		bookChapterVerse := blue(fmt.Sprintf("%s %d:%d", hit.Source.Book, hit.Source.Chapter, hit.Source.Verse))
		score := magenta(fmt.Sprintf("[%.4f]", hit.Score))

		// Highlight all instances of the search term within the verse text
		verseTextHighlighted := searchTermRegex.ReplaceAllStringFunc(hit.Source.Text, func(match string) string {
			return green(match) // Apply green color to each match
		})

		fmt.Printf("%s %s %s\n", bookChapterVerse, verseTextHighlighted, score)
	}
}
