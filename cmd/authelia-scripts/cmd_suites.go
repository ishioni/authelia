package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/clems4ever/authelia/internal/suites"
	"github.com/clems4ever/authelia/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ErrNotAvailableSuite error raised when suite is not available.
var ErrNotAvailableSuite = errors.New("unavailable suite")

// ErrNoRunningSuite error raised when no suite is running
var ErrNoRunningSuite = errors.New("no running suite")

// runningSuiteFile name of the file containing the currently running suite
var runningSuiteFile = ".suite"

var headless bool
var onlyForbidden bool

func init() {
	SuitesTestCmd.Flags().BoolVar(&headless, "headless", false, "Run tests in headless mode")
	SuitesTestCmd.Flags().BoolVar(&onlyForbidden, "only-forbidden", false, "Mocha 'only' filters are forbidden")
}

// SuitesListCmd Command for listing the available suites
var SuitesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available suites.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(strings.Join(listSuites(), "\n"))
	},
	Args: cobra.ExactArgs(0),
}

// SuitesSetupCmd Command for setuping a suite environment
var SuitesSetupCmd = &cobra.Command{
	Use:   "setup [suite]",
	Short: "Setup a Go suite environment. Suites can be listed using the list command.",
	Run: func(cmd *cobra.Command, args []string) {
		providedSuite := args[0]
		runningSuite, err := getRunningSuite()

		if err != nil {
			log.Fatal(err)
		}

		if runningSuite != "" && runningSuite != providedSuite {
			log.Fatal("A suite is already running")
		}

		if err := setupSuite(providedSuite); err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.ExactArgs(1),
}

// SuitesTeardownCmd Command for tearing down a suite environment
var SuitesTeardownCmd = &cobra.Command{
	Use:   "teardown [suite]",
	Short: "Teardown a Go suite environment. Suites can be listed using the list command.",
	Run: func(cmd *cobra.Command, args []string) {
		var suiteName string
		if len(args) == 1 {
			suiteName = args[0]
		} else {
			runningSuite, err := getRunningSuite()

			if err != nil {
				panic(err)
			}

			if runningSuite == "" {
				panic(ErrNoRunningSuite)
			}
			suiteName = runningSuite
		}

		if err := teardownSuite(suiteName); err != nil {
			panic(err)
		}
	},
	Args: cobra.MaximumNArgs(1),
}

// SuitesTestCmd Command for testing a suite
var SuitesTestCmd = &cobra.Command{
	Use:   "test [suite]",
	Short: "Test a suite. Suites can be listed using the list command.",
	Run:   testSuite,
	Args:  cobra.MaximumNArgs(1),
}

func listSuites() []string {
	suiteNames := make([]string, 0)
	for _, k := range suites.GlobalRegistry.Suites() {
		suiteNames = append(suiteNames, k)
	}
	sort.Strings(suiteNames)
	return suiteNames
}

func checkSuiteAvailable(suite string) error {
	suites := listSuites()

	for _, s := range suites {
		if s == suite {
			return nil
		}
	}
	return ErrNotAvailableSuite
}

func runSuiteSetupTeardown(command string, suite string) error {
	selectedSuite := suite
	err := checkSuiteAvailable(selectedSuite)

	if err != nil {
		if err == ErrNotAvailableSuite {
			log.Fatal(errors.New("Suite named " + selectedSuite + " does not exist"))
		}
		log.Fatal(err)
	}

	s := suites.GlobalRegistry.Get(selectedSuite)

	cmd := utils.CommandWithStdout("go", "run", "cmd/authelia-suites/main.go", command, selectedSuite)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return utils.RunCommandWithTimeout(cmd, s.SetUpTimeout)
}

func setupSuite(suiteName string) error {
	log.Infof("Setup environment for suite %s...", suiteName)
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	interrupted := false

	go func() {
		<-signalChannel
		interrupted = true
	}()

	if errSetup := runSuiteSetupTeardown("setup", suiteName); errSetup != nil || interrupted {
		err := teardownSuite(suiteName)
		if err != nil {
			log.Error(err)
		}
		return errSetup
	}

	return nil
}

func teardownSuite(suiteName string) error {
	log.Infof("Tear down environment for suite %s...", suiteName)
	return runSuiteSetupTeardown("teardown", suiteName)
}

func testSuite(cmd *cobra.Command, args []string) {
	runningSuite, err := getRunningSuite()
	if err != nil {
		log.Fatal(err)
	}

	if len(args) == 1 {
		suite := args[0]

		if runningSuite != "" && suite != runningSuite {
			log.Fatal(errors.New("Running suite (" + runningSuite + ") is different than suite to be tested (" + suite + "). Shutdown running suite and retry"))
		}

		if err := runSuiteTests(suite, runningSuite == ""); err != nil {
			log.Fatal(err)
		}
	} else {
		if runningSuite != "" {
			fmt.Println("Running suite (" + runningSuite + ") detected. Run tests of that suite")
			if err := runSuiteTests(runningSuite, false); err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Println("No suite provided therefore all suites will be tested")
			if err := runAllSuites(); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func getRunningSuite() (string, error) {
	exist, err := utils.FileExists(runningSuiteFile)

	if err != nil {
		return "", err
	}

	if !exist {
		return "", nil
	}

	b, err := ioutil.ReadFile(runningSuiteFile)
	return string(b), err
}

func runSuiteTests(suiteName string, withEnv bool) error {
	if withEnv {
		if err := setupSuite(suiteName); err != nil {
			return err
		}
	}

	suite := suites.GlobalRegistry.Get(suiteName)

	// Default value is 1 minute
	timeout := "60s"
	if suite.TestTimeout > 0 {
		timeout = fmt.Sprintf("%ds", int64(suite.TestTimeout/time.Second))
	}
	testCmdLine := fmt.Sprintf("go test ./internal/suites -timeout %s -run '^(Test%sSuite)$'", timeout, suiteName)

	log.Infof("Running tests of suite %s...", suiteName)
	log.Debugf("Running tests with command: %s", testCmdLine)

	cmd := utils.CommandWithStdout("bash", "-c", testCmdLine)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if headless {
		cmd.Env = append(cmd.Env, "HEADLESS=y")
	}

	testErr := cmd.Run()

	if withEnv {
		err := teardownSuite(suiteName)

		if err != nil {
			log.Error(err)
		}
	}

	return testErr
}

func runAllSuites() error {
	log.Info("Start running all suites")
	for _, s := range listSuites() {
		if err := runSuiteTests(s, true); err != nil {
			return err
		}
	}
	log.Info("All suites passed successfully")
	return nil
}
