package testutils

import (
	"os"
	"path/filepath"

	"github.com/juju/errors"
)

// GetTestCaseDirs scans the dir recursively and returns relative paths
// to all the dirs which contain the file "test_case.yaml". For example:
//
// []string{"mytest1_foo", "some_group/mytest1", "some_group/mytest2"}
func GetTestCaseDirs(testCasesDir string, testDescrFname string) ([]string, error) {
	var result []string

	// Walk the directory recursively
	err := filepath.Walk(testCasesDir, func(path string, info os.FileInfo, err error) error {
		// If there's an error walking, wrap and return it
		if err != nil {
			return errors.Annotate(err, "failed to walk path: "+path)
		}

		// If it's a directory and contains testDescrFname
		if info.IsDir() {
			// Check if testDescrFname exists in the current directory
			testCaseFile := filepath.Join(path, testDescrFname)
			if _, err := os.Stat(testCaseFile); err == nil {
				// If the file exists, add the relative path to the result
				relPath, err := filepath.Rel(testCasesDir, path)
				if err != nil {
					return errors.Annotate(err, "failed to get relative path for: "+path)
				}
				// Add the relative path to the result
				result = append(result, relPath)
			}
		}
		return nil
	})

	if err != nil {
		return nil, errors.Annotate(err, "failed to scan directories")
	}

	return result, nil
}
