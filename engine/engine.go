package engine

import (
	"database/sql"
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devplayg/agent_manager/event"
	"github.com/devplayg/golibs/network"
	_ "github.com/go-sql-driver/mysql"
)

var (
	stats = expvar.NewMap("engine")
	db    *sql.DB
)

const (
	YYYYMMDDHH24MISS = "2006-01-02 15:04:05"
)

type Batcher struct {
	duration time.Duration
	size     int
	datadir  string

	c chan *event.Agent
}

func init() {
	//	log.Critical("crit")
	//initDatabase("root:sniper123!@#@tcp(aptxa:3306)/aptxam?charset=utf8&allowAllFiles=true")
}

func NewBatcher(duration time.Duration, size, maxpending int, datadir string) *Batcher {
	return &Batcher{
		duration: duration,
		size:     size,
		datadir:  datadir,
		c:        make(chan *event.Agent, maxpending),
	}
}

func (this *Batcher) Start(errChan chan<- error, debug *bool) error {
	// Create data directory
	if _, err := os.Stat(this.datadir); os.IsNotExist(err) {
		os.Mkdir(this.datadir, 0755)
	}

	go func() {
		timer := time.NewTimer(this.duration)
		timer.Stop()

		queue := make([]*event.Agent, 0, this.size)
		save := func() {
			count := int64(len(queue))
			stats.Add("eventsCollected", count)

			_, err := insert(queue)
			if err == nil {

			} else {
				//				writeToFile(queue)
				stats.Add("insertFailed", 1)
				errChan <- err
				return
			}
			if *debug {
				log.Printf("Collected: %d (Total: %s)", count, stats.Get("eventsCollected").String())
			}

			queue = make([]*event.Agent, 0, this.size)

		}

		for {
			select {
			case event := <-this.c:
				if *debug {
					log.Printf("Data")
				}
				queue = append(queue, event)
				if len(queue) == 1 {
					timer.Reset(this.duration)
				}

				if len(queue) == this.size {
					timer.Stop()
					save()
				}

			case <-timer.C: // Timeout
				save()
			}
		}

	}()
	return nil
}

func (b *Batcher) C() chan<- *event.Agent {
	return b.c
}

//func writeToFile(queue []*event.Agent) {
//	fp := "temp/" + time.Now().Format(LOGFILETIME) + "0.log"
//	f, _ := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
//	defer f.Close()

//	for _, a := range queue {
//		q := fmt.Sprintf("%s\t%d\t%.1f\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
//			a.Guid,
//			network.IpToInt32(a.IP),
//			a.OsVersionNumber,
//			a.OsBit,
//			a.OsIsServer,
//			a.ComputerName,
//			a.Eth,
//			a.FullPolicyVersion,
//			a.TodayPolicyVersion,
//			a.Rdate.Format(YYYYMMDDHH24MISS),
//			a.Udate.Format(YYYYMMDDHH24MISS),
//		)
//		f.WriteString(q)
//	}
//}

func insert(queue []*event.Agent) (sql.Result, error) {
	var arr []string
	query_prefix := "insert into ast_agent(guid, ip, os_version_number, os_bit, os_is_server, computer_name, eth, full_policy_version, today_policy_version, rdate, udate) values "
	query_suffix := " on duplicate key update ip = values(ip),os_version_number = values(os_version_number),os_bit = values(os_bit),os_is_server = values(os_is_server),computer_name = values(computer_name),eth = values(eth),full_policy_version = values(full_policy_version),today_policy_version = values(today_policy_version),udate = values(udate) "

	for _, a := range queue {
		q := fmt.Sprintf("('%s',%d,%.1f,%d,%d,'%s','%s','%s','%s','%s','%s')",
			a.Guid,
			network.IpToInt32(a.IP),
			a.OsVersionNumber,
			a.OsBit,
			a.OsIsServer,
			a.ComputerName,
			a.Eth,
			a.FullPolicyVersion,
			a.TodayPolicyVersion,
			a.Rdate.Format(YYYYMMDDHH24MISS),
			a.Udate.Format(YYYYMMDDHH24MISS),
		)
		arr = append(arr, q)
	}
	query := query_prefix + strings.Join(arr, ",") + query_suffix

	return db.Exec(query)
}

func saveAsFile(datadir string, queue []*event.Agent) (*os.File, error) {
	// Write the data in the queue to a file
	tmpfile, err := ioutil.TempFile(datadir, "syslog_"+time.Now().Format("20060102_150405_"))
	defer tmpfile.Close()
	if err != nil {
		return tmpfile, err
	}
	//	var str string
	//	for _, r := range queue {
	//		str += fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s\t%d\t%d\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
	//			r.Data["timestamp"].(time.Time).Format(YYYYMMDDHH24MISS), // timestamp
	//			r.Rdate.Format(YYYYMMDDHH24MISS),                         // rdate
	//			libs.IpToInt32(r.Addr.IP),                                // src_ip
	//			r.Addr.Port,                                              // port
	//			r.Data["hostname"].(string),                              // hostname
	//			r.Data["proc_id"].(string),                               // proc_id
	//			r.Data["facility"].(int),                                 // facility
	//			r.Data["severity"].(int),                                 // severity
	//			r.Data["app_name"].(string),                              // app_name
	//			r.Data["priority"].(int),                                 // priority
	//			r.Data["version"].(int),                                  // version
	//			r.Data["msg_id"].(string),                                // msg_id
	//			r.Data["message"].(string),                               // message
	//			r.Data["structured_data"].(string),                       // structured_data
	//			r.Origin, // origin
	//		)
	//	}

	//	if _, err := tmpfile.WriteString(str); err != nil {
	//		return tmpfile, err
	//	}

	// Write data in file to database
	return tmpfile, nil
}

func InitDatabase(dataSourceName string) error {
	var err error
	db, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}

	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)
	return nil
}
