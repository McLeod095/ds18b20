package common

import (
	"time"
	"fmt"
	"io/ioutil"
	"strconv"
)

type Sensor struct {
	ID	string
	Average	float64
	values	[]Dimension
	Max	int
	Path	string
}

func (s Sensor) String() string {
	if len(s.values) > 0 {
		return fmt.Sprintf("%s %.2f (%d)[%d]", s.ID, s.Average/1000.0, s.values[len(s.values)-1].Value, len(s.values))
	}else{
		return fmt.Sprintf("%s %.2f (0)[%d]", s.ID, s.Average/1000.0, len(s.values))
	}
}

func (s *Sensor) Update() (err error) {
	content, err := ioutil.ReadFile(s.Path)
	if err != nil {
		return
	}

	value, err := strconv.Atoi(string(content[69:len(content)-1]))
	if err != nil {
		return
	}

	d := Dimension{Timestamp: time.Now(), Value: value}
	s.values = append(s.values, d)
	if len(s.values) > s.Max {
		s.values = s.values[1:]
	}

	sum := 0
	for _, v := range s.values {
		sum += v.Value
	}
	s.Average = float64(sum)/float64(len(s.values))
	return
}

func (s Sensor) Last() (last Dimension) {
	if len(s.values) > 0 {
		last = s.values[len(s.values)-1]
	}
	return
}
