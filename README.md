# crawler

web crawler!

## setup

- Nix Flake (without metrics)
  1. Run `nix develop`
  2. Start `valkey-server`
  3. (Optional) Run `protoc -I=. --go_out=protos/ protos_raw/`
  4. go run .
- Docker Compose
  1. Run `docker compose up`
- Raw
  1. Install everything within the packages section of `flake.nix`
  2. Follow the Nix Flake section, excluding step 1

## options

### no section

#### initial_pages = []string

initial urls to crawl. default: [ "arxiv.org" ]

#### data_path = string

path to store data. default: "data/"

#### log_level = string

log level. default: "info"

#### queue_prioritization = string

method for the queue to be sorted. default: "mean"

#### user_agent = string

user agent. default: "Mozilla/5.0 (compatible; Crawler/1.0; +[http://www.google.com/bot.html](http://www.google.com/bot.html))"

#### workers = int

workers to use in pipelines. default: 50

#### batch_size = int

amount of urls to crawl before saving to valkey,
and starting new pipeline. default: 100

#### recover = bool

wether the database is cleared on start or not. default: true

#### crawl_timeout = duration

time before canceling crawl. default: 5s

#### default_crawl_delay = duration

delay between crawling hosts. default: 500ms

#### respect_robots = bool

wether to respect /robots.txt or not. default: true

### services section

#### valkey_addr = string

address to valkey-server. default: "localhost:6379"

#### enable_pyroscope = bool

wether to monitor performance via pyroscope or not. default: false

#### pyroscope_uri = string

uri to the pyroscope server. default: "[http://localhost:4040](http://localhost:4040)"

#### enable_metrics = bool

wether to monitor metrics via prometheus or not. default: false

#### prometheus_push_addr = string

address to the prometheus push server. default: ":9091"
