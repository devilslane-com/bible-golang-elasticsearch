# Elasticsearch Primer

## Library

### Create the Elasticsearch Docker container 

```docker
docker network create elastic
docker run --name elasticsearch --net elastic -p 9200:9200 -it -m 4GB elasticsearch:8.12.0
```

Check the logs for the HTTP basic auth user `elastic`.

Next, create the index with a `PUT` request found in `resources/index/create_index_with_analyzer.json`.

## Import

### Build the importer

Add the ES connection details and *password* for connecting to `https://localhost:9200`.

```bash
cd src/elasticbible/import
go build -o ../../../bin/import.exe .
```

### Run the importer

```bash
cd bin/
./import -file="../resources/data/en_bbe.json" -host="https://localhost:9200" -username="elastic" -password="somepassword" -index="bible"
```

## Search

### Build the searcher

```bash
cd src/elasticbible/search
go build -o ../../../bin/search.exe .
```

### Run the searcher

Set some env vars for convenience:

```bash
export ES_HOST=https://localhost:9200
export ES_INDEX=bible
export ES_USERNAME=elastic
export ES_PASSWORD=somepassword
export MAX_RESULTS=20
```

```bash
cd bin/
./search -host="https://localhost:9200"  -index="bible" -username="elastic" -password="somepassword" -text="adam" -max=30
```