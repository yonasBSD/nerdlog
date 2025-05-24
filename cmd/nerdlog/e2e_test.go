package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dimonomid/nerdlog/core"
	"github.com/dimonomid/nerdlog/core/testutils"
	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const e2eTestOutputRoot = "/tmp/nerdlog_e2e_test_output"
const e2eTestScenarioYamlFname = "test_scenario.yaml"

type E2ETestScenarioYaml struct {
	Descr string `yaml:"descr"`

	ScenarioParams E2EScenarioParams `yaml:"scenario_params"`
	TestSteps      []E2ETestStep     `yaml:"test_steps"`
}

// E2ETestConfigLogStream converts to ConfigLogStream (from config.go)
type E2ETestConfigLogStream struct {
	LogFiles testutils.TestCaseLogfiles `yaml:"log_files"`

	Options core.ConfigLogStreamOptions `yaml:"options"`
}

type E2EScenarioParams struct {
	ConfigLogStreams map[string]E2ETestConfigLogStream `yaml:"config_log_streams"`
	TerminalSize     TerminalSize                      `yaml:"terminal_size"`
}

type TerminalSize struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
}

type E2ETestStep struct {
	// Descr is a human-readable step description
	Descr string `yaml:"descr"`

	DelayBefore time.Duration `yaml:"delay_before"`

	// SendKeys will be passed verbatim to "tmux send-keys"
	SendKeys []string `yaml:"send_keys"`

	WantScreenSnapshot *WantScreenSnapshot `yaml:"want_screen_snapshot"`
	WantScreenContains *WantScreenContains `yaml:"want_screen_contains"`
}

type WantScreenSnapshot struct {
	// SnapshotFname is the filename of the snapshot.
	SnapshotFname string `yaml:"snapshot"`

	// Substitutions defines which substitutions we need to make to the snapshot
	// before comparing it.
	Substitutions []SnapshotSubstitution `yaml:"substitutions"`
}

type WantScreenContains struct {
	// Substring is the substring to look for on the screen.
	Substring string `yaml:"substring"`

	// If PeriodicSendKeys is not nil, we'll send some keys periodically.
	PeriodicSendKeys *PeriodSendKeys `yaml:"periodic_send_keys"`
}

type PeriodSendKeys struct {
	SendKeys []string      `yaml:"send_keys"`
	Period   time.Duration `yaml:"period"`
}

type SnapshotSubstitution struct {
	// Pattern is the regexp
	Pattern string `yaml:"pattern"`
	// Replacement is the string to replace matching regexps with.
	Replacement string `yaml:"replacement"`
}

func applySubstitutions(s string, substitutions []SnapshotSubstitution) string {
	for _, sub := range substitutions {
		re := regexp.MustCompile(sub.Pattern)
		s = re.ReplaceAllString(s, sub.Replacement)
	}
	return s
}

func TestE2EScenarios(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to get caller info")
	}

	// Get directory of the current file
	parentDir := filepath.Dir(filename)
	testScenariosDir := filepath.Join(parentDir, "e2e_testdata", "test_scenarios")

	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))

	if err := os.MkdirAll(e2eTestOutputRoot, 0755); err != nil {
		t.Fatalf("unable to create e2e test output root dir %s: %s", e2eTestOutputRoot, err.Error())
	}

	testScenarioDirs, err := testutils.GetTestCaseDirs(testScenariosDir, e2eTestScenarioYamlFname)
	if err != nil {
		t.Fatalf("unable to create e2e test case dirs: %s", err.Error())
	}

	// Find out the nerdlog binary: use what NERDLOG_E2E_TEST_NERDLOG_BINARY env
	// var tells us, or if there's no such env var, then build the current package.
	nerdlogBinary := os.Getenv("NERDLOG_E2E_TEST_NERDLOG_BINARY")
	if nerdlogBinary == "" {
		nerdlogBinary = filepath.Join(e2eTestOutputRoot, "nerdlog")

		fmt.Printf("No NERDLOG_E2E_TEST_NERDLOG_BINARY env var, building nerdlog to %s\n", nerdlogBinary)
		if err := runGoBuild(parentDir, nerdlogBinary); err != nil {
			t.Fatalf("failed to build nerdlog binary: %s", err.Error())
		}
	}

	for _, testName := range testScenarioDirs {
		t.Run(testName, func(t *testing.T) {
			tsCtx := &e2eTestScenarioContext{
				nerdlogBinary: nerdlogBinary,

				testName:        testName,
				testScenarioDir: filepath.Join(testScenariosDir, testName),
				testOutputDir:   filepath.Join(e2eTestOutputRoot, testName),
				repoRoot:        repoRoot,
			}

			if err := runE2ETestScenario(t, tsCtx); err != nil {
				t.Fatalf("running e2e test scenario %s: %s", testName, err.Error())
			}
		})
	}
}

type e2eTestScenarioContext struct {
	nerdlogBinary string

	testName        string
	testScenarioDir string
	testOutputDir   string
	repoRoot        string
}

func runE2ETestScenario(t *testing.T, tsCtx *e2eTestScenarioContext) error {
	testScenarioDescrFname := filepath.Join(tsCtx.testScenarioDir, e2eTestScenarioYamlFname)

	if err := os.RemoveAll(tsCtx.testOutputDir); err != nil {
		return errors.Annotatef(err, "unable to remove test output dir %s", tsCtx.testOutputDir)
	}

	if err := os.MkdirAll(tsCtx.testOutputDir, 0755); err != nil {
		return errors.Annotatef(err, "unable to create test output dir %s", tsCtx.testOutputDir)
	}

	data, err := os.ReadFile(testScenarioDescrFname)
	if err != nil {
		return errors.Annotatef(err, "reading yaml test case descriptor %s", testScenarioDescrFname)
	}

	var tc E2ETestScenarioYaml
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return errors.Annotatef(err, "unmarshaling yaml from %s", testScenarioDescrFname)
	}

	e2eTH, err := newE2ETestHelper(tc.ScenarioParams, tsCtx)
	if err != nil {
		return errors.Annotatef(err, "creating E2ETestHelper")
	}

	defer e2eTH.Close()

	snapshotErrors := []error{}
	for i, step := range tc.TestSteps {
		stepSID := fmt.Sprintf("%.2d_%s", i+1, testutils.Slug(step.Descr))
		fmt.Printf("Running step %s\n", stepSID)

		stepOutputDir := filepath.Join(tsCtx.testOutputDir, "steps", stepSID)
		if err := os.MkdirAll(stepOutputDir, 0755); err != nil {
			return errors.Annotatef(err, "creating lstream dir %s", stepSID)
		}

		assertArgs := []interface{}{"test case %s", stepSID}

		if step.DelayBefore > 0 {
			fmt.Printf("Sleeping %s...\n", step.DelayBefore)
			time.Sleep(step.DelayBefore)
		}

		if len(step.SendKeys) > 0 {
			if err := e2eTH.tmuxSendKeys(step.SendKeys...); err != nil {
				return errors.Trace(err)
			}
		}

		if snapshot := step.WantScreenSnapshot; snapshot != nil {
			wantSnapshotFname := filepath.Join(tsCtx.testScenarioDir, snapshot.SnapshotFname)

			err = os.WriteFile(filepath.Join(stepOutputDir, "want_snapshot_filename.txt"), []byte(snapshot.SnapshotFname), 0644)
			if err != nil {
				return errors.Annotatef(err, "test step #%d: writing want_snapshot_filename.txt", i)
			}

			gotSnapshotFname := filepath.Join(stepOutputDir, "got_snapshot.txt")
			timeout := 5 * time.Second // TODO: we might want specify it in the step.
			if err := e2eTH.waitForSnapshot(
				wantSnapshotFname,
				gotSnapshotFname,
				snapshot.Substitutions,
				timeout,
			); err != nil {
				// Don't return just yet, just remember that there was an error, and
				// keep going. This is to facilitate updting multiple outputs at once
				// (using "make update-test-expectations") when we're doing some legit
				// changes to the output.
				snapshotErrors = append(snapshotErrors, errors.Trace(err))
				fmt.Printf("Got an error: %s\n", err.Error())
			} else {
				fmt.Printf("Snapshots matched\n")
			}
		}

		if wantContains := step.WantScreenContains; wantContains != nil {
			gotSnapshotFname := filepath.Join(stepOutputDir, "got_snapshot_nonexact.txt")
			timeout := 5 * time.Second // TODO: we might want specify it in the step.
			if err := e2eTH.waitForScreenContains(
				wantContains,
				gotSnapshotFname,
				timeout,
			); err != nil {
				return errors.Trace(err)
			} else {
				fmt.Printf("Snapshot contains %s\n", wantContains.Substring)
			}
		}

		assert.Equal(t, 1, 1, assertArgs...)
	}

	if len(snapshotErrors) > 0 {
		return errors.Errorf("got %d snapshot errors: %+v", len(snapshotErrors), snapshotErrors)
	}

	return nil
}

func newE2ETestHelper(
	params E2EScenarioParams,
	tsCtx *e2eTestScenarioContext,
) (*E2ETestHelper, error) {
	cfgLogStreams := make(core.ConfigLogStreams, len(params.ConfigLogStreams))
	for lstreamName, testCfg := range params.ConfigLogStreams {
		resolved, err := testutils.ResolveLogfiles(tsCtx.testScenarioDir, &testCfg.LogFiles)
		if err != nil {
			return nil, errors.Annotatef(err, "resolving logfiles")
		}

		testOutputLstreamDir := filepath.Join(tsCtx.testOutputDir, "lstreams", lstreamName)
		if err := os.MkdirAll(testOutputLstreamDir, 0755); err != nil {
			return nil, errors.Annotatef(err, "creating lstream dir %s", lstreamName)
		}

		provisioned, err := testutils.ProvisionLogFiles(
			resolved,
			testOutputLstreamDir,
			tsCtx.repoRoot,
		)
		if err != nil {
			return nil, errors.Annotatef(err, "provisioning logfiles")
		}

		options := testCfg.Options
		for _, envVar := range provisioned.ExtraEnv {
			options.ShellInit = append(options.ShellInit, fmt.Sprintf("export %s", envVar))
		}

		cfgLogStreams[lstreamName] = core.ConfigLogStream{
			Hostname: "localhost",
			LogFiles: []string{
				provisioned.LogfileLast,
				provisioned.LogfilePrev,
			},
			Options: options,
		}
	}

	testOutputLstreamsCfgFname := filepath.Join(tsCtx.testOutputDir, "log_streams.yaml")

	cfgLogStreamsFull := ConfigLogStreams{
		LogStreams: cfgLogStreams,
	}

	data, err := yaml.Marshal(cfgLogStreamsFull)
	if err != nil {
		return nil, errors.Annotatef(err, "marshaling yaml for logstreams config")
	}

	if err := os.WriteFile(testOutputLstreamsCfgFname, data, 0644); err != nil {
		return nil, errors.Annotatef(err, "saving logstreams yaml config as %s", testOutputLstreamsCfgFname)
	}

	e2eTH := &E2ETestHelper{
		tsCtx:                      *tsCtx,
		tmuxSessionName:            "nerdlog-e2e-test",
		testOutputLstreamsCfgFname: testOutputLstreamsCfgFname,
	}

	// Just in case the session is alive from some previous uncleanly-finished run,
	// kill it, ignoring the errors.
	e2eTH.tmuxKillSession()

	if err := e2eTH.tmuxNewSession(); err != nil {
		return nil, errors.Trace(err)
	}

	if err := e2eTH.tmuxResizeWindow(params.TerminalSize.X, params.TerminalSize.Y); err != nil {
		// Now that we've already created a session, we need to close it before
		// returning an error.
		e2eTH.Close()

		return nil, errors.Trace(err)
	}

	return e2eTH, nil
}

type E2ETestHelper struct {
	tmuxSessionName            string
	testOutputLstreamsCfgFname string

	tsCtx e2eTestScenarioContext
}

func (e2eTH *E2ETestHelper) tmuxNewSession() error {
	fmt.Printf("Starting a new tmux session %s\n", e2eTH.tmuxSessionName)
	if _, err := e2eTH.tmuxCmd("new-session", "-d", "-s", e2eTH.tmuxSessionName); err != nil {
		return errors.Annotatef(err, "starting new tmux session")
	}

	return nil
}

func (e2eTH *E2ETestHelper) tmuxResizeWindow(x, y int) error {
	fmt.Printf("Resizing tmux window to %d, %d\n", x, y)
	_, err := e2eTH.tmuxCmd(
		"resize-window",
		"-t", fmt.Sprintf("%s:0", e2eTH.tmuxSessionName),
		"-x", strconv.Itoa(x),
		"-y", strconv.Itoa(y),
	)
	if err != nil {
		return errors.Annotatef(err, "resizing tmux window")
	}

	return nil
}

func (e2eTH *E2ETestHelper) varsubst(s string) string {
	s = strings.Replace(s, "${NERDLOG_BINARY}", e2eTH.tsCtx.nerdlogBinary, -1)
	s = strings.Replace(s, "${NERDLOG_LOGSTREAMS_CONFIG_FILE}", e2eTH.testOutputLstreamsCfgFname, -1)
	s = strings.Replace(s, "${NERDLOG_TEST_OUTPUT_DIR}", e2eTH.tsCtx.testOutputDir, -1)
	return s
}

func (e2eTH *E2ETestHelper) varsubstSlice(ss []string) []string {
	ret := make([]string, 0, len(ss))
	for _, s := range ss {
		ret = append(ret, e2eTH.varsubst(s))
	}
	return ret
}

// waitForSnapshot keeps checking tmux snapshot (saving it to the output file),
// and returns nil once they match. If they don't match after a timeout,
// returns an error.
func (e2eTH *E2ETestHelper) waitForSnapshot(
	wantSnapshotFname string,
	gotSnapshotFname string,
	substitutions []SnapshotSubstitution,
	timeout time.Duration,
) error {
	wantData, err := os.ReadFile(wantSnapshotFname)
	if err != nil {
		return errors.Annotatef(err, "reading wanted snapshot data from %s", wantSnapshotFname)
	}

	start := time.Now()
	for time.Since(start) < timeout {
		if err := e2eTH.tmuxCapturePane(gotSnapshotFname, substitutions); err != nil {
			return errors.Trace(err)
		}

		gotData, err := os.ReadFile(gotSnapshotFname)
		if err != nil {
			return errors.Annotatef(err, "reading actual snapshot data from %s", gotSnapshotFname)
		}

		wantStr := string(wantData)
		gotStr := string(gotData)

		if wantStr == gotStr {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return errors.Errorf("snapshot data is not equal")
}

// waitForScreenContains keeps checking tmux snapshot, and returns nil once
// it contains the given substr
func (e2eTH *E2ETestHelper) waitForScreenContains(
	wantScreenContains *WantScreenContains,
	gotSnapshotFname string,
	timeout time.Duration,
) error {
	start := time.Now()
	lastSendKeys := time.Now()
	for time.Since(start) < timeout {
		if err := e2eTH.tmuxCapturePane(gotSnapshotFname, nil); err != nil {
			return errors.Trace(err)
		}

		gotData, err := os.ReadFile(gotSnapshotFname)
		if err != nil {
			return errors.Annotatef(err, "reading actual snapshot data from %s", gotSnapshotFname)
		}

		if strings.Contains(string(gotData), wantScreenContains.Substring) {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
		if wantScreenContains.PeriodicSendKeys != nil {
			psk := wantScreenContains.PeriodicSendKeys
			if time.Since(lastSendKeys) >= psk.Period {
				fmt.Printf("Sending periodic keys: %+v\n", psk.SendKeys)
				lastSendKeys = time.Now()

				if err := e2eTH.tmuxSendKeys(psk.SendKeys...); err != nil {
					return errors.Trace(err)
				}
			}
		}

	}

	return errors.Errorf("snapshot never contained %s", wantScreenContains.Substring)
}

func (e2eTH *E2ETestHelper) tmuxSendKeys(keys ...string) error {
	// Replace various placeholders in the keys
	keys = e2eTH.varsubstSlice(keys)

	keysData, _ := json.Marshal(keys)
	fmt.Printf("Sending keys to tmux session: %s\n", keysData)
	_, err := e2eTH.tmuxCmd(
		append(
			[]string{
				"send-keys",
				"-t", fmt.Sprintf("%s:0", e2eTH.tmuxSessionName),
			},
			keys...,
		)...,
	)
	if err != nil {
		return errors.Annotatef(err, "tmux send-keys")
	}

	return nil
}

func (e2eTH *E2ETestHelper) tmuxKillSession() error {
	fmt.Printf("Killing tmux session %s\n", e2eTH.tmuxSessionName)
	_, err := e2eTH.tmuxCmd(
		"kill-session",
		"-t", e2eTH.tmuxSessionName,
	)
	if err != nil {
		return errors.Annotatef(err, "killing tmux session %s", e2eTH.tmuxSessionName)
	}

	return nil
}

func (e2eTH *E2ETestHelper) tmuxCapturePane(
	targetFname string,
	substitutions []SnapshotSubstitution,
) error {
	fmt.Printf("Capturing tmux pane into %s\n", targetFname)
	res, err := e2eTH.tmuxCmd(
		"capture-pane",
		"-pt", fmt.Sprintf("%s:0.0", e2eTH.tmuxSessionName),
	)
	if err != nil {
		return errors.Annotatef(err, "capturing tmux pane")
	}

	data := applySubstitutions(res.stdout, substitutions)

	if err := os.WriteFile(targetFname, []byte(data), 0644); err != nil {
		return errors.Annotatef(err, "writing captured tmux pane to %s", targetFname)
	}

	return nil
}

type cmdRunResult struct {
	stdout string
	stderr string
}

func (e2eTH *E2ETestHelper) tmuxCmd(parts ...string) (*cmdRunResult, error) {
	cmd := exec.Command("tmux", parts...)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := &cmdRunResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}

	if err != nil {
		return nil, errors.Annotatef(err, "calling tmux cli: %s %s", res.stdout, res.stderr)
	}

	return res, nil
}

func (e2eTH *E2ETestHelper) Close() error {
	if err := e2eTH.tmuxKillSession(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func runGoBuild(packageDir, targetBinary string) error {
	cmd := exec.Command("go", "build", "-o", targetBinary, packageDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Annotatef(err, "building Go package %s as %s: %s", packageDir, targetBinary, string(output))
	}

	return nil
}
