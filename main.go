package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	M = iota
	D
)

const (
	// sampleUnit is the sample interval (the time  between checks)
	sampleUnit = 1 * time.Minute

	// sampleDefault is the total number of samples (in sampleUnit) that is
	// considered when determining windowDefault
	sampleDefault = 30

	// windowDefault is the minimal number of samples with wired LAN inside the
	// sampleDefault last samples.
	windowDefault = 5

	// configFile is the config file path (absolute path)
	configFile = "iflandown.toml"
)

// etherPrefixes are usual linux kernel ethernet interface names.
var etherPrefixes = []string{"en", "eth"}

// Check contains the result of a query to the linux kernel sys
type Check struct {
	t      time.Time
	isDown bool
}

func (c Check) String() string {
	state := "🆙"
	if c.isDown {
		state = "❌"
	}

	return fmt.Sprintf("📆 %s %s", c.t.Format("2006-01-02 15:04:05"), state)
}

// MonitorChecks contains the last Check
type MonitorChecks []Check

type Config struct {
	Period   int
	Window   int
	Commands [][]string
}

var conf Config

var scriptStartTime time.Time = time.Now().UTC()

var checks MonitorChecks

// availableInterfaces returns all net interfaces
func availableInterfaces() ([]string, error) {

	interfaces, err := net.Interfaces()

	if err != nil {
		return nil, err
	}

	var res []string

	for _, i := range interfaces {
		res = append(res, i.Name)
	}

	return res, nil
}

func filterEthernetInterfaces(ifs []string) []string {
	var res []string
	for _, i := range ifs {
		for _, prefix := range etherPrefixes {
			if strings.HasPrefix(i, prefix) {
				res = append(res, i)
				break
			}
		}
	}

	return res
}

// isDown queries /sys/class/net/<iface>/carrier
//
// A value of 0 or inexistant path is
// signal of no ethernet.
// A value of 1 means "up".
func isDown(iName string) bool {

	path := fmt.Sprintf("/sys/class/net/%s/carrier", iName)
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return true
	}

	if err != nil {

		return true
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return true
	}

	text := strings.TrimSpace(string(content))

	i, err := strconv.ParseInt(text, 10, 32)

	if err != nil {
		return true
	}

	if i == 0 {
		return true
	}

	return false
}

// monitor checks the wired LAN interface and saves the result in checks
// It also removes old entries in the checks slice
func monitor() {
	log(M, "Starting monitor goroutine at %s", scriptStartTime.Format("2006-01-02 15:04:05"))
	ticker := time.NewTicker(1 * sampleUnit)
	for range ticker.C {

		ifs, err := availableInterfaces()
		if err != nil {
			continue
		}

		ethernetIfs := filterEthernetInterfaces(ifs)

		currentT := time.Now().UTC()

		is := true
		for _, name := range ethernetIfs {
			if !isDown(name) {
				is = false
			}
		}

		check := Check{t: currentT, isDown: is}
		checks = append(checks, check)
		log(M, "Adding check for %s at %s. Number of checks is %d", ethernetIfs, check, len(checks))

		// delete elements of slice
		if len(checks) > period()+10 {
			checks = checks[10:]
		}
	}
}

// decide executes the configured commands if there is no minimum number of
// samples with ethernet in the last Checks.
// It stops the daemon after that.
func decide() {
	log(D, "Starting decide goroutine at %s", scriptStartTime.Format("2006-01-02 15:04:05"))
	ticker := time.NewTicker(1 * sampleUnit)

	for range ticker.C {
		samplesDown := 0
		currentT := time.Now().UTC()
		log(D, "Deciding if enough downtime at %s", currentT.Format("2006-01-02 15:04:05"))

		// do not decide if not enough runtime
		if !runEnoughTime(currentT) {
			continue
		}

		// How many samples are down
		t := currentT
		var firstSample, lastSample string
		for i := 1; i <= period(); i++ {
			t = getNextSample(t)
			if i == 1 {
				firstSample = t.Format("2006-01-02 15:04:05")
			}

			if isSampleDown(t) {
				samplesDown++
			}

			if i == period() {
				lastSample = t.Format("2006-01-02 15:04:05")
			}
		}

		log(D, "⏮  '%s', '%s' ⏭ ", firstSample, lastSample)
		log(D, "Samples: %d ❌  down, %d 🆙 up. Period %d , window %d", samplesDown, period()-samplesDown, period(), window())

		if period()-samplesDown < window() {
			err := executeScripts()
			if err != nil {
				log(D, "Error executing commands. %s. Exiting.", err)
				os.Exit(1)
			}

			log(D, "successfully run commands. %s", "Exiting")
			os.Exit(0)
		}
	}
}

func runEnoughTime(t time.Time) bool {
	diff := t.Sub(scriptStartTime)

	// 1 sample period till the first ticker
	requiredDuration := (time.Duration(period()) + 1) * sampleUnit
	log(D, "%s running. We need at least %s", diff, requiredDuration)

	return diff >= requiredDuration
}

func log(t int, format string, keysAndValues ...interface{}) {
	logTmpl := "[👀] %s\n"
	if t == D {
		logTmpl = "[🤛] %s\n"
	}

	msg := fmt.Sprintf(format, keysAndValues...)

	fmt.Printf(logTmpl, msg)
}

func minuteLabel(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

func getNextSample(c time.Time) time.Time {

	t := c.Add(-1 * sampleUnit)
	return minuteLabel(t)
}

// period returns the sample period.
func period() int {
	if conf.Period > 0 {
		return conf.Period
	}

	return sampleDefault
}

func window() int {
	if conf.Window > 0 {
		return conf.Window
	}

	return windowDefault
}

func isSampleDown(t time.Time) bool {
	for _, c := range checks {
		minute := minuteLabel(c.t)
		if t.Equal(minute) {
			return c.isDown
		}
	}

	return true
}

func executeScripts() error {

	for _, c := range conf.Commands {
		fmt.Printf("Executing command %s\n", c)
		cmd := exec.Command(c[0], c[1:]...)
		out, err := cmd.CombinedOutput()
		fmt.Printf("Combined command output:\n%s\n", string(out))
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {

	var noCheck bool
	flag.BoolVar(&noCheck, "nocheck", false, "Run commands, without checks.")
	flag.Parse()

	// Read conf file
	fmt.Printf("Reading conf file %s\n", configFile)
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		fmt.Printf("Error %s\n", err)
		os.Exit(1)
	}

	if noCheck {
		err := executeScripts()
		if err != nil {
			log(D, "Error executing commands. %s. Exiting.", err)
			os.Exit(1)
		}

		log(D, "successfully run commands. %s", "Exiting")
		os.Exit(0)
	}

	go monitor()
	go decide()
	select {}
}
