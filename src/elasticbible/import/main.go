package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
)

// VerseData represents the structure of each verse document to be imported.
type BookData struct {
	Abbrev   string     `json:"abbrev"`
	Chapters [][]string `json:"chapters"`
}

func main() {
	var (
		jsonFilePath, esHost, esUsername, esPassword, indexName string
	)

	flag.StringVar(&jsonFilePath, "file", "", "Path to the JSON file containing Bible data")
	flag.StringVar(&esHost, "host", "https://localhost:9200", "Elasticsearch host")
	flag.StringVar(&esUsername, "username", "", "Elasticsearch username")
	flag.StringVar(&esPassword, "password", "", "Elasticsearch password")
	flag.StringVar(&indexName, "index", "bible", "Elasticsearch index name")
	flag.Parse()

	if jsonFilePath == "" {
		log.Fatal("JSON file path was empty. Usage: ./import -file='path/to/your/bible_data.json'")
	}

	data, err := ioutil.ReadFile(jsonFilePath)
	if err != nil {
		log.Fatalf("Failed to read JSON file: %s", err)
	}

	// Check for and remove the BOM if present
	bom := []byte{0xEF, 0xBB, 0xBF}
	if bytes.HasPrefix(data, bom) {
		data = data[len(bom):] // Strip BOM
	}

	var booksData []BookData
	if err := json.Unmarshal(data, &booksData); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %s", err)
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

	bulkIndexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         indexName,
		Client:        es,
		NumWorkers:    4,
		FlushBytes:    int(5e+6),
		FlushInterval: 30,
	})
	if err != nil {
		log.Fatalf("Error creating the bulk indexer: %s", err)
	}

	for _, book := range booksData {
		for chapterIndex, verses := range book.Chapters {
			for verseIndex, verseText := range verses {
				verseNum := verseIndex + 1 // Verse numbers typically start at 1
				docID := fmt.Sprintf("%s-%d-%d", book.Abbrev, chapterIndex+1, verseNum)
				doc := map[string]interface{}{
					"abbrev":  book.Abbrev,
					"chapter": chapterIndex + 1,
					"verse":   verseNum,
					"en":      verseText,
				}

				data, err := json.Marshal(doc)
				if err != nil {
					log.Printf("Could not encode document %s: %s", docID, err)
					continue
				}

				err = bulkIndexer.Add(
					context.Background(),
					esutil.BulkIndexerItem{
						Action:     "index",
						DocumentID: docID,
						Body:       bytes.NewReader(data),
						OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, resp esutil.BulkIndexerResponseItem) {
							// Handle success
						},
						OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, resp esutil.BulkIndexerResponseItem, err error) {
							// Handle error
							log.Printf("Failed to index document %s: %s", item.DocumentID, err)
						},
					},
				)
				if err != nil {
					log.Fatalf("Error adding document to bulk indexer: %s", err)
				}
			}
		}
	}

	if err := bulkIndexer.Close(context.Background()); err != nil {
		log.Fatalf("Error closing bulk indexer: %s", err)
	}

	stats := bulkIndexer.Stats()
	if stats.NumFailed > 0 {
		log.Printf("Indexed %d documents with %d failures\n", stats.NumFlushed, stats.NumFailed)
	} else {
		log.Printf("Successfully indexed %d documents\n", stats.NumFlushed)
	}
}
