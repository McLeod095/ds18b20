package main

import (
	"bufio"
	"ds18b20/common"
	"ds18b20/models"
	//	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func RootHandler(tpl *template.Template, db *models.Mysql) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Request:", r)
		if tpl == nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		sensors, err := db.GetSensors()
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}

		err = tpl.Execute(w, sensors)
		if err != nil {
			fmt.Println(err)
			http.Error(w, http.StatusText(404), 404)
			return
		}
	})
}

func HistoryHandler(db *models.Mysql) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Request /hist:", r)

		sensor := r.FormValue("sensor")
		if len(sensor) == 0 {
			return
		}
		dimensions, err := db.History(sensor)
		if err != nil {
			return
		}

		val := "?(["
		s := ""
		for _, d := range dimensions {
			val += fmt.Sprintf("[Date(\"%s\"),%f]%s\n", d.Timestamp, float64(d.Value)/1000.0, s)
			//val += "[Date(" + string(d.Timestamp) + ")," + d.Value/1000.0 + "]" + s + "\n"
			s = ","
		}
		val += "]);"
		//	b, err := json.Marshal(dimensions)
		//	if err != nil {
		//		return
		//	}
		fmt.Fprintf(w, "%s", val)
	})
}

func EventHandler(b *common.Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Request:", r)
		flusher, ok := w.(http.Flusher)

		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Accel-Buffering", "No")

		clientChan := make(chan []byte)
		b.Add(clientChan)

		defer func() {
			b.Delete(clientChan)
		}()

		notify := w.(http.CloseNotifier).CloseNotify()

		go func() {
			<-notify
			b.Delete(clientChan)
		}()

		for {
			fmt.Fprintf(w, "%s\n\n", <-clientChan)
			flusher.Flush()
		}
	})
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
	db := models.NewDB(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
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

	indexTpl, err := template.New("").ParseGlob(path.Join("templates", "*.html"))
	if err != nil {
		panic(err)
	}

	broker := common.NewBroker()
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
				eventString := fmt.Sprintf("event: %s\ndata: %s", v.ID, v.Json())
				broker.Event([]byte(eventString))
			}
			fmt.Println("---------------------")
		}
	}()

	//	router := httprouter.New()
	//	router.GET("/", RootHandler)
	//	http.ListenAndServe(":8088", router)
	http.Handle("/", RootHandler(indexTpl.Lookup("index.html"), db))
	http.Handle("/event", EventHandler(broker))
	http.Handle("/history", HistoryHandler(db))
	http.Handle("/api/sensors", SensorsHandler(db))
	http.Handle("/api/sensor/:id", SensorHandler(db))
	http.ListenAndServe(":8088", nil)
}
