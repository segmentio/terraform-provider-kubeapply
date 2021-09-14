package util

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

type confirmResponse struct {
	responseStr string
	err         error
}

// Confirm gets a yes/no confirmation from the user on the command-line before continuing.
func Confirm(ctx context.Context, prompt string, skip bool) (bool, error) {
	fmt.Printf("%s (yes/no) ", prompt)

	if skip {
		log.Infof("Automatically answering yes because skip is set to true")
		return true, nil
	}

	responseChan := make(chan confirmResponse, 1)

	go func() {
		var responseStr string
		_, err := fmt.Scanln(&responseStr)
		responseChan <- confirmResponse{
			responseStr: responseStr,
			err:         err,
		}
	}()

	select {
	case response := <-responseChan:
		if response.err != nil {
			log.Warnf(
				"Got error reading response, not continuing: %+v",
				response.err,
			)
			return false, response.err
		}
		if r := strings.TrimSpace(strings.ToLower(response.responseStr)); r != "y" && r != "yes" {
			log.Infof("Not continuing")
			return false, nil
		}
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}
