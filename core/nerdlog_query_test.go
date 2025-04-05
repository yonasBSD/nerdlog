package core

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const testOutputRoot = "/tmp/nerdlog_query_test_output"

type TestCaseYaml struct {
	Descr    string           `yaml:"descr"`
	Logfiles TestCaseLogfiles `yaml:"logfiles"`
	Args     []string         `yaml:"args"`
}

type TestCaseLogfiles struct {
	Kind LogfilesKind `yaml:"kind"`
	Dir  string       `yaml:"dir"`
}

type LogfilesKind string

const (
	LogfilesKindAllFromDir LogfilesKind = "all_from_dir"
)

var AllLogfilesKinds = map[LogfilesKind]struct{}{
	LogfilesKindAllFromDir: {},
}

func TestReadFileRelativeToThisFile(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to get caller info")
	}

	// Get directory of the current file
	parentDir := filepath.Dir(filename)
	testCasesDir := filepath.Join(parentDir, "nerdlog_query_testdata", "test_cases")
	nerdlogQuerySh := filepath.Join(parentDir, "nerdlog_query.sh")

	if err := os.MkdirAll(testOutputRoot, 0755); err != nil {
		t.Fatalf("unable to create test output root dir %s: %s", testOutputRoot, err.Error())
	}

	entries, err := os.ReadDir(testCasesDir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			if err := runTestCase(t, nerdlogQuerySh, testCasesDir, entry.Name()); err != nil {
				t.Fatalf("running test case %s: %s", entry.Name(), err.Error())
			}
		})
	}
}

func runTestCase(t *testing.T, nerdlogQuerySh, testCasesDir, testName string) error {
	testCaseDir := filepath.Join(testCasesDir, testName)
	testCaseDescrFname := filepath.Join(testCaseDir, "test_case.yaml")

	assertArgs := []interface{}{"test case %s", testName}

	testOutputDir := filepath.Join(testOutputRoot, testName)
	if err := os.MkdirAll(testOutputDir, 0755); err != nil {
		return errors.Annotatef(err, "unable to create test output dir %s", testOutputDir)
	}

	data, err := os.ReadFile(testCaseDescrFname)
	if err != nil {
		return errors.Annotatef(err, "reading yaml test case descriptor %s", testCaseDescrFname)
	}

	var tc TestCaseYaml
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return errors.Annotatef(err, "unmarshaling yaml from %s", testCaseDescrFname)
	}

	logfiles, err := resolveLogfiles(testCaseDir, &tc.Logfiles)
	if err != nil {
		return errors.Annotatef(err, "resolving logfiles")
	}

	if len(logfiles) != 2 {
		return errors.Errorf(
			"For now, there must be exactly 2 logfiles, but got %d: %v",
			len(logfiles), logfiles,
		)
	}

	logfileLast := filepath.Join(testOutputDir, "logfile")
	logfilePrev := filepath.Join(testOutputDir, "logfile.1")

	if err := copyFile(logfiles[0], logfileLast); err != nil {
		return errors.Annotatef(err, "copying logfile last: from %s to %s", logfiles[0], logfileLast)
	}

	if err := copyFile(logfiles[1], logfilePrev); err != nil {
		return errors.Annotatef(err, "copying logfile prev: from %s to %s", logfiles[1], logfilePrev)
	}

	indexFname := filepath.Join(testOutputDir, "nerdlog_query_index")
	stdoutFname := filepath.Join(testOutputDir, "nerdlog_query_stdout")
	stderrFname := filepath.Join(testOutputDir, "nerdlog_query_stderr")

	os.Remove(indexFname)
	os.Remove(stdoutFname)
	os.Remove(stderrFname)

	stdoutFile, err := os.Create(stdoutFname)
	stderrFile, err := os.Create(stderrFname)

	cmdArgs := append(
		[]string{
			nerdlogQuerySh,
			"--logfile-last", logfileLast,
			"--logfile-prev", logfilePrev,
			"--cache-file", indexFname,
		},
		tc.Args...,
	)

	cmd := exec.Command("/bin/bash", cmdArgs...)

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	fmt.Printf("Running %+v\n", cmdArgs)
	if err := cmd.Run(); err != nil {
		return errors.Annotatef(err, "running nerdlog query command %+v", cmdArgs)
	}

	wantStdout, err := os.ReadFile(filepath.Join(testCaseDir, "want_stdout"))
	if err != nil {
		return errors.Annotatef(err, "reading want_stdout")
	}

	wantStderr, err := os.ReadFile(filepath.Join(testCaseDir, "want_stderr"))
	if err != nil {
		return errors.Annotatef(err, "reading want_stderr")
	}

	gotStdout, err := os.ReadFile(stdoutFname)
	if err != nil {
		return errors.Annotatef(err, "reading %s", stdoutFname)
	}

	gotStderr, err := os.ReadFile(stderrFname)
	if err != nil {
		return errors.Annotatef(err, "reading %s", stderrFname)
	}

	assert.Equal(t, string(wantStdout), string(gotStdout), assertArgs...)
	assert.Equal(t, string(wantStderr), string(gotStderr), assertArgs...)

	// TODO: somehow also test with incomplete index: maybe just remove lines one by
	// one from the end (until the "prevlog_lines" line), rerunning the same command,
	// and making sure that the stdout is the same (stderr might differ though).

	// TODO: the stats lines in stdout (these starting from "s:") are printed in
	// arbitrary order because they come from a hashmap, so simply comparing the
	// output is a bad idea. Gotta do some post-processing, like sorting these "s:"
	// lines, before comparing them.

	// TODO: gotta also do some benchmarks. Very useful for refactorings.

	return nil
}

func resolveLogfiles(
	testCaseDir string, logfilesDescr *TestCaseLogfiles,
) ([]string, error) {
	switch logfilesDescr.Kind {
	case LogfilesKindAllFromDir:
		logfilesDir := filepath.Join(testCaseDir, logfilesDescr.Dir)

		entries, err := os.ReadDir(logfilesDir)
		if err != nil {
			return nil, errors.Annotatef(err, "reading logfiles dir %q", logfilesDir)
		}

		var ret []string
		for _, entry := range entries {
			if entry.IsDir() {
				return nil, errors.Errorf("a dir %q in the logfiles dir %q", entry.Name(), logfilesDir)
			}

			ret = append(ret, filepath.Join(logfilesDir, entry.Name()))
		}

		sort.Strings(ret)

		return ret, nil
	default:
		return nil, errors.Errorf("invalid logfiles kind %q", logfilesDescr.Kind)
	}
}

func copyFile(srcPath, destPath string) error {
	// Open the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return errors.Annotatef(err, "opening source file")
	}
	defer srcFile.Close()

	// Create the destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return errors.Annotatef(err, "creating destination file")
	}
	defer destFile.Close()

	// Copy the contents of the source file to the destination file
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.Annotatef(err, "copying data")
	}

	// Ensure the destination file is written to disk
	err = destFile.Sync()
	if err != nil {
		return errors.Annotatef(err, "syncing destination file")
	}

	return nil
}
