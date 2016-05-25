package main

import (
	"bufio"
	"ds18b20/common"
	"ds18b20/models"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}

func SensorsHandler(db *models.Mysql) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, http.StatusText(405), 405)
			return
		}

		sensors, err := db.GetSensors()
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}

		for _, v := range sensors {
			fmt.Fprintf(w, "%s\n", v)
		}
	})
}

func SensorHandler(db *models.Mysql) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, http.StatusText(405), 405)
			return
		}

		dimension, err := db.GetLast("28-021500d05fff")
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}

		fmt.Fprintf(w, "%s\n", dimension)
	})
}

func discovery(dpath string) (sensors []*common.Sensor, err error) {
	fpath := path.Join(dpath, "w1_bus_master1/w1_master_slaves")
	master, err := os.Open(fpath)
	if err != nil {
		return
	}
	defer master.Close()

	reader := bufio.NewReader(master)
	for {
		str, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		str = strings.TrimSpace(str)
		sensors = append(sensors, &common.Sensor{ID: str, Max: 10, Path: path.Join(dpath, str, "w1_slave")})
	}
	return
}

func main() {
	w1dir := "/sys/bus/w1/devices"
	db := models.NewDB(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8",
		"ds18b20",
		"ds18b20",
		"127.0.0.1",
		"3306",
		"ds18b20",
	))

	sensors, err := discovery(w1dir)
	if err != nil {
		panic(err)
	}

	checktime := time.Tick(10 * time.Second)
	printtime := time.Tick(10 * time.Second)

	go func() {
		for _ = range checktime {
			for _, v := range sensors {
				go func(s *common.Sensor) {
					if err := s.Update(); err == nil {
						db.AddTemp(s.ID, s.Last())
					}
				}(v)
			}
		}
	}()

	go func() {
		for _ = range printtime {
			for _, v := range sensors {
				fmt.Printf("%s last - %s\n", v, v.Last())
			}
			fmt.Println("---------------------")
		}
	}()
	http.HandleFunc("/", RootHandler)
	http.Handle("/api/sensors", SensorsHandler(db))
	http.Handle("/api/sensor/:id", SensorHandler(db))
	http.ListenAndServe(":8088", nil)
}
