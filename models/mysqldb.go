package	models

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"ds18b20/common"
)

type Mysql struct {
	db	*sql.DB
}

func NewDB(dsn string) *Mysql {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	return &Mysql{db}
}

func (m *Mysql) Close() {
	m.db.Close()
}

func (m *Mysql) AddTemp(id string, v common.Dimension) (err error){
	const query = "INSERT INTO ds18b20 (sensor_id, timestamp, value) VALUES(?,?,?)"

	stmt, err := m.db.Prepare(query)
	if err != nil {
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(
		id,
		v.Timestamp.Unix(),
		v.Value,
	)
	return
}

func (m *Mysql) GetAllForSensor(s string) (dimensions []*common.Dimension, err error) {
	const query = "SELECT timestamp, value FROM ds18b20 WHERE sensor_id=?"

	rows, err := m.db.Query(query, s)
	if err != nil {
		return
	}

	for rows.Next() {
		dim := common.NewDimension()
		err = rows.Scan(&dim.Timestamp, &dim.Value)
		if err != nil {
			return nil, err
		}
		dimensions = append(dimensions, dim)
	}
	return
}

func (m *Mysql) GetSensors() (sensors []string, err error) {
	const query = "SELECT DISTINCT(sensor_id) FROM ds18b20"

	rows,err := m.db.Query(query)
	if err != nil {
		return
	}

	for rows.Next(){
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		sensors = append(sensors, s)
	}
	return
}
