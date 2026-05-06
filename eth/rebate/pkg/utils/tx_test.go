package utils_test

import (
	"rebate/pkg/utils"
	"testing"
)

func TestCreateTx(t *testing.T) {

	data, err := utils.CreateTestTx()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)

}
