package testutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/juju/errors"
)

type TestCaseLogfiles struct {
	Kind LogfilesKind `yaml:"kind"`

	// Dir is only relevant for LogfilesKindAllFromDir
	Dir string `yaml:"dir"`

	// JournalctlDataFile is only relevant for LogfilesKindJournalctl
	JournalctlDataFile string `yaml:"journalctl_data_file"`
}

type LogfilesKind string

const (
	LogfilesKindAllFromDir LogfilesKind = "all_from_dir"
	LogfilesKindJournalctl LogfilesKind = "journalctl"
)

var AllLogfilesKinds = map[LogfilesKind]struct{}{
	LogfilesKindAllFromDir: {},
	LogfilesKindJournalctl: {},
}

type ResolvedLogFiles struct {
	// If files is not empty, we need to use these files.
	Files []string

	// If journalctlDataFile is not empty, we need to use that file
	// as the data for mocked journalctl.
	JournalctlDataFile string
}

func ResolveLogfiles(
	testCaseDir string, logfilesDescr *TestCaseLogfiles,
) (*ResolvedLogFiles, error) {
	switch logfilesDescr.Kind {
	case LogfilesKindAllFromDir:
		logfilesDir := filepath.Join(testCaseDir, logfilesDescr.Dir)

		entries, err := os.ReadDir(logfilesDir)
		if err != nil {
			return nil, errors.Annotatef(err, "reading logfiles dir %q", logfilesDir)
		}

		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				return nil, errors.Errorf("a dir %q in the logfiles dir %q", entry.Name(), logfilesDir)
			}

			files = append(files, filepath.Join(logfilesDir, entry.Name()))
		}

		sort.Strings(files)

		return &ResolvedLogFiles{
			Files: files,
		}, nil

	case LogfilesKindJournalctl:
		if logfilesDescr.JournalctlDataFile == "" {
			return nil, errors.Errorf("kind is journalctl, but JournalctlDataFile is empty")
		}

		return &ResolvedLogFiles{
			JournalctlDataFile: filepath.Join(
				testCaseDir, logfilesDescr.JournalctlDataFile,
			),
		}, nil

	default:
		return nil, errors.Errorf("invalid logfiles kind %q", logfilesDescr.Kind)
	}
}

type ProvisionedLogFiles struct {
	LogfileLast, LogfilePrev string

	// extraEnv contains extra env vars in the format "VARIABLE=VALUE"
	ExtraEnv []string
}

func ProvisionLogFiles(resolved *ResolvedLogFiles, testOutputDir, repoRoot string) (*ProvisionedLogFiles, error) {
	var logfileLast, logfilePrev string

	// extraEnv contains extra env vars in the format "VARIABLE=VALUE"
	var extraEnv []string

	if len(resolved.Files) > 0 {
		logfiles := resolved.Files
		if len(logfiles) == 0 || len(logfiles) > 2 {
			return nil, errors.Errorf(
				"For now, there must be exactly 1 or 2 logfiles, but got %d: %v",
				len(logfiles), logfiles,
			)
		}

		logfileLast = filepath.Join(testOutputDir, "logfile")
		logfilePrev = filepath.Join(testOutputDir, "logfile.1")

		if err := CopyFile(logfiles[0], logfileLast); err != nil {
			return nil, errors.Annotatef(err, "copying logfile last: from %s to %s", logfiles[0], logfileLast)
		}

		if err := setSyslogFileModTime(logfileLast); err != nil {
			return nil, errors.Trace(err)
		}

		if len(logfiles) > 1 {
			if err := CopyFile(logfiles[1], logfilePrev); err != nil {
				return nil, errors.Annotatef(err, "copying logfile prev: from %s to %s", logfiles[1], logfilePrev)
			}

			if err := setSyslogFileModTime(logfilePrev); err != nil {
				return nil, errors.Trace(err)
			}
		}
	} else if resolved.JournalctlDataFile != "" {
		srcJournalctlMockDir := filepath.Join(repoRoot, "cmd", "journalctl_mock")
		srcJournalctlMockShFname := filepath.Join(srcJournalctlMockDir, "journalctl_mock.sh")
		srcJournalctlMockGoFname := filepath.Join(srcJournalctlMockDir, "journalctl_mock.go")
		srcJournalctlMockGoModFname := filepath.Join(srcJournalctlMockDir, "go.mod")
		srcJournalctlMockGoSumFname := filepath.Join(srcJournalctlMockDir, "go.sum")

		tgtJournalctlMockDir := filepath.Join(testOutputDir, "journalctl_mock")
		tgtJournalctlMockShFname := filepath.Join(tgtJournalctlMockDir, "journalctl_mock.sh")
		tgtJournalctlMockGoFname := filepath.Join(tgtJournalctlMockDir, "journalctl_mock.go")
		tgtJournalctlMockGoModFname := filepath.Join(tgtJournalctlMockDir, "go.mod")
		tgtJournalctlMockGoSumFname := filepath.Join(tgtJournalctlMockDir, "go.sum")

		if err := os.MkdirAll(tgtJournalctlMockDir, 0755); err != nil {
			return nil, errors.Errorf("unable to create journalctl_mock output dir %s: %s", tgtJournalctlMockDir, err.Error())
		}

		if err := CopyFile(srcJournalctlMockShFname, tgtJournalctlMockShFname); err != nil {
			return nil, errors.Annotatef(err, "copying from %s to %s", srcJournalctlMockShFname, tgtJournalctlMockShFname)
		}
		if err := os.Chmod(tgtJournalctlMockShFname, 0755); err != nil {
			return nil, errors.Annotatef(err, "changing permissions for journalctl mock %s", tgtJournalctlMockShFname)
		}

		if err := CopyFile(srcJournalctlMockGoFname, tgtJournalctlMockGoFname); err != nil {
			return nil, errors.Annotatef(err, "copying from %s to %s", srcJournalctlMockGoFname, tgtJournalctlMockGoFname)
		}

		if err := CopyFile(srcJournalctlMockGoModFname, tgtJournalctlMockGoModFname); err != nil {
			return nil, errors.Annotatef(err, "copying from %s to %s", srcJournalctlMockGoModFname, tgtJournalctlMockGoModFname)
		}

		if err := CopyFile(srcJournalctlMockGoSumFname, tgtJournalctlMockGoSumFname); err != nil {
			return nil, errors.Annotatef(err, "copying from %s to %s", srcJournalctlMockGoSumFname, tgtJournalctlMockGoSumFname)
		}

		// Special case for the journalctl, no need to copy any files.
		logfileLast = "journalctl"
		logfilePrev = "journalctl"
		extraEnv = append(
			extraEnv,
			fmt.Sprintf("NERDLOG_JOURNALCTL_MOCK=%s", tgtJournalctlMockShFname),
			fmt.Sprintf("NERDLOG_JOURNALCTL_MOCK_DATA=%s", resolved.JournalctlDataFile),
		)
	} else {
		return nil, errors.Errorf(
			"For now, there must be exactly 1 or 2 logfiles, or journalctl data file, but got nothing",
		)
	}

	return &ProvisionedLogFiles{
		LogfileLast: logfileLast,
		LogfilePrev: logfilePrev,
		ExtraEnv:    extraEnv,
	}, nil
}

func CopyFile(srcPath, destPath string) error {
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

// setSyslogFileModTime takes the path to a syslog file, and sets its modification
// time to the timestamp of the last log message from that file.
func setSyslogFileModTime(fname string) error {
	lastLogTime, err := getLatestSyslogTimestamp(fname)
	if err != nil {
		return errors.Annotatef(err, "getting timestamp of the last log message in %s", fname)
	}

	if err := os.Chtimes(fname, lastLogTime, lastLogTime); err != nil {
		return errors.Annotatef(err, "setting mod time of %s", fname)
	}

	return nil
}

// Function to extract the latest timestamp from a syslog file
func getLatestSyslogTimestamp(filePath string) (time.Time, error) {
	// Open the syslog file
	file, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	// Regular expression to match a typical syslog timestamp (e.g., "Apr  5 14:33:22")
	re := regexp.MustCompile(`^([A-Za-z]{3} \s?\d{1,2} \d{2}:\d{2}:\d{2})`)

	var latestTimestamp time.Time

	// Read the file backwards (find the last line)
	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lastLine = scanner.Text() // Keep updating lastLine until the last line
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, err
	}

	// Extract the timestamp from the last line
	matches := re.FindStringSubmatch(lastLine)
	if len(matches) > 0 {
		timestampStr := matches[1]
		latestTimestamp, err = time.ParseInLocation("Jan 2 15:04:05", timestampStr, time.UTC)
		if err != nil {
			return time.Time{}, err
		}

		latestTimestamp = setYear(latestTimestamp, time.Now().Year())
	}

	return latestTimestamp, nil
}

func setYear(t time.Time, year int) time.Time {
	// Return a new time.Time with the desired year
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}
