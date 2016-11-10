package ssh

import (
	"encoding/base64"
	"fmt"

	"github.com/pkg/errors"

	"github.com/howeyc/gopass"

	"github.com/havoc-io/mutagen/environment"
	"github.com/havoc-io/mutagen/process"
)

const (
	PrompterEnvironmentVariable              = "MUTAGEN_PROMPTER"
	PrompterContextBase64EnvironmentVariable = "MUTAGEN_PROMPTER_CONTEXT_BASE64"
)

type PromptClass uint8

const (
	PromptClassSecret PromptClass = iota
	PromptClassDisplay
	PromptClassBinary
)

func ClassifyPrompt(prompt string) PromptClass {
	// TODO: Implement using white-listing regexes based on known OpenSSH
	// prompts.
	return PromptClassSecret
}

func PromptCommandLine(context, prompt string) (string, error) {
	// Classify the prompt.
	class := ClassifyPrompt(prompt)

	// Figure out which getter to use.
	var getter func() ([]byte, error)
	if class == PromptClassDisplay || class == PromptClassBinary {
		getter = gopass.GetPasswdEchoed
	} else {
		getter = gopass.GetPasswd
	}

	// Print the context (if any) and the prompt.
	if context != "" {
		fmt.Println(context)
	}
	fmt.Print(prompt)

	// Get the result.
	result, err := getter()
	if err != nil {
		return "", errors.Wrap(err, "unable to read response")
	}

	// Success.
	return string(result), nil
}

func prompterEnvironment(prompter, context string) []string {
	// If there is no prompter, return nil to just use the current environment.
	if prompter == "" {
		return nil
	}

	// Convert context to base64 encoding so that we can pass it through the
	// environment safely.
	contextBase64 := base64.StdEncoding.EncodeToString([]byte(context))

	// Create a copy of the current environment.
	result := make(map[string]string, len(environment.Current))
	for k, v := range environment.Current {
		result[k] = v
	}

	// Insert necessary environment variables.
	result["SSH_ASKPASS"] = process.Current.ExecutablePath
	result["DISPLAY"] = "mutagen"
	result[PrompterEnvironmentVariable] = prompter
	result[PrompterContextBase64EnvironmentVariable] = contextBase64

	// Convert into the desired format.
	return environment.Format(result)
}