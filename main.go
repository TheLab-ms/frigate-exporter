package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx"
	"github.com/kelseyhightower/envconfig"
)

const migration = `
CREATE TABLE IF NOT EXISTS motion (
	id text primary key,
	camera text not null,
	start timestamp not null,
	stop timestamp not null
);

CREATE INDEX IF NOT EXISTS idx_motion_camera ON motion (camera);
CREATE INDEX IF NOT EXISTS idx_motion_start ON motion (start);
CREATE INDEX IF NOT EXISTS idx_motion_stop ON motion (stop);
`

type config struct {
	FrigateURL string   `required:"true" split_words:"true"`
	Cameras    []string `required:"true"`

	PostgresHost     string `required:"true" split_words:"true"`
	PostgresUser     string `default:"postgres" split_words:"true"`
	PostgresPassword string `required:"true" split_words:"true"`

	ScrapeInterval time.Duration `default:"1h" split_words:"true"`
}

func main() {
	conf := &config{}
	if err := envconfig.Process("", conf); err != nil {
		panic(err)
	}

	db, err := pgx.Connect(pgx.ConnConfig{
		Host:     conf.PostgresHost,
		User:     conf.PostgresUser,
		Password: conf.PostgresPassword,
	})
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(migration)
	if err != nil {
		panic(err)
	}

	runLoop(conf.ScrapeInterval, func() (ok bool) {
		ok = true
		for _, camera := range conf.Cameras {
			if err := scrapeCamera(db, conf.FrigateURL, camera); err != nil {
				log.Printf("error scraping camera %q: %s", camera, err)
				ok = false
			}
		}
		return ok
	})
}

func scrapeCamera(db *pgx.Conn, baseURL, cameraName string) error {
	start := time.Now()
	defer log.Printf("finished scraping motion events for camera %q in %s", cameraName, time.Since(start))

	var queryStart time.Time
	err := db.QueryRow("SELECT stop FROM motion WHERE camera = $1 ORDER BY stop DESC LIMIT 1", cameraName).Scan(&queryStart)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("finding cursor position: %s", err)
	}
	log.Printf("last timestamp for camera %q = %s", cameraName, queryStart)

	events, err := listEvents(baseURL, cameraName, queryStart)
	if err != nil {
		return err
	}
	for _, event := range events {
		_, err := db.Exec("INSERT INTO motion (id, camera, start, stop) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", event.ID, cameraName, time.Unix(int64(event.StartTime), 0), time.Unix(int64(event.EndTime), 0))
		if err != nil {
			return fmt.Errorf("inserting motion event into database: %s", err)
		}
		log.Printf("inserted event %s for camera %q", event.ID, cameraName)
	}

	return nil
}

var httpClient = &http.Client{Timeout: time.Second * 30}

func listEvents(baseURL, cameraName string, start time.Time) ([]*event, error) {
	resp, err := httpClient.Get(fmt.Sprintf("%s/api/%s/recordings?after=%d", baseURL, cameraName, start.Unix()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	events := []*event{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}

	return events, nil
}

type event struct {
	ID        string  `json:"id"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
}

func runLoop(interval time.Duration, fn func() bool) {
	var lastRetry time.Duration
	for {
		if fn() {
			lastRetry = 0
			time.Sleep(interval)
			continue
		}

		if lastRetry == 0 {
			lastRetry = time.Millisecond * 250
		}
		lastRetry += lastRetry / 5
		if lastRetry > time.Hour {
			lastRetry = time.Hour
		}
		time.Sleep(lastRetry)
	}
}
