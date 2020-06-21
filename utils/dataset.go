package utils

import "fmt"

const (
	datasetPrefix = "dlkit-dataset"
)

func GeneralDatasetPVC(datasetName string) string {
	return fmt.Sprintf("%s-%s", datasetPrefix, datasetName)
}
