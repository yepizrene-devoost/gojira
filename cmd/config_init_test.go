package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/secretprompt"
)

const initToken = "ATATT3xFfGF0-very-secret-token-0123456789"

type fakeTester struct {
	info string
	err  error
}

func (f fakeTester) TestConnection() (string, error) { return f.info, f.err }

// initRecorder replaces every side effect of `config init`, so no test touches the real
// keychain, the real config file, the network or a terminal.
type initRecorder struct {
	terminal    bool
	secret      string
	secretErr   error
	secretCalls []string
	savedConfig *config.Config
	savedToken  string
	tokenSaves  int
	tester      fakeTester
	clientErr   error
}

func (r *initRecorder) deps() configInitDeps {
	return configInitDeps{
		isTerminal: func() bool { return r.terminal },
		readSecret: func(prompt string) (string, error) {
			r.secretCalls = append(r.secretCalls, prompt)
			return r.secret, r.secretErr
		},
		saveConfig: func(c *config.Config) error { r.savedConfig = c; return nil },
		saveToken:  func(token string) error { r.savedToken = token; r.tokenSaves++; return nil },
		newClient: func() (connectionTester, error) {
			if r.clientErr != nil {
				return nil, r.clientErr
			}
			return r.tester, nil
		},
	}
}

func invokeConfigInit(t *testing.T, r *initRecorder, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	command := newConfigInitCommand(r.deps())
	var out, errOut bytes.Buffer
	command.SetIn(strings.NewReader(stdin))
	command.SetOut(&out)
	command.SetErr(&errOut)
	command.SetArgs(args)
	command.SilenceUsage = true
	command.SilenceErrors = true
	err = command.Execute()
	return out.String(), errOut.String(), err
}

func assertTokenNeverPrinted(t *testing.T, stdout, stderr string) {
	t.Helper()
	for name, stream := range map[string]string{"stdout": stdout, "stderr": stderr} {
		if strings.Contains(stream, initToken) || strings.Contains(stream, initToken[:8]) {
			t.Fatalf("the token leaked to %s: %q", name, stream)
		}
	}
}

func assertNothingSaved(t *testing.T, r *initRecorder) {
	t.Helper()
	if r.savedConfig != nil || r.tokenSaves != 0 {
		t.Fatalf("nothing should be saved, got config=%v tokenSaves=%d", r.savedConfig, r.tokenSaves)
	}
}

func TestConfigInitMasksTheTokenOnATerminal(t *testing.T) {
	r := &initRecorder{terminal: true, secret: initToken, tester: fakeTester{info: "https://x (Jira 1)"}}
	stdout, stderr, err := invokeConfigInit(t, r, "mycompany.atlassian.net\nme@example.com\n")
	if err != nil {
		t.Fatal(err)
	}

	if len(r.secretCalls) != 1 || r.secretCalls[0] != "Jira API token: " {
		t.Fatalf("the token must be read once through the masked prompt, calls: %q", r.secretCalls)
	}
	if r.savedToken != initToken {
		t.Fatalf("saved token = %q", r.savedToken)
	}
	if r.savedConfig == nil || r.savedConfig.Domain != "mycompany.atlassian.net" || r.savedConfig.Email != "me@example.com" {
		t.Fatalf("saved config = %+v", r.savedConfig)
	}
	for _, want := range []string{"Jira domain (e.g. mycompany.atlassian.net): ", "Jira email: "} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("the %q prompt changed: %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "Jira API token") {
		t.Fatalf("on a terminal the masked prompt draws its own label, config init must not print a second one: %q", stdout)
	}
	assertTokenNeverPrinted(t, stdout, stderr)
}

func TestConfigInitReadsALineWhenThereIsNoTerminal(t *testing.T) {
	// Previous behaviour, pinned: pipes and scripts keep feeding three lines.
	r := &initRecorder{terminal: false, tester: fakeTester{info: "https://x (Jira 1)"}}
	stdout, stderr, err := invokeConfigInit(t, r, "mycompany.atlassian.net\nme@example.com\n"+initToken+"\n")
	if err != nil {
		t.Fatal(err)
	}

	if len(r.secretCalls) != 0 {
		t.Fatalf("without a terminal there is nothing to mask, masked prompt calls: %q", r.secretCalls)
	}
	if r.savedToken != initToken {
		t.Fatalf("saved token = %q", r.savedToken)
	}
	if !strings.Contains(stdout, "Jira API token: ") {
		t.Fatalf("the token prompt changed: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr must stay empty, got %q", stderr)
	}
	assertTokenNeverPrinted(t, strings.ReplaceAll(stdout, "Jira API token: ", ""), stderr)
}

func TestConfigInitTokenStdinReadsTheWholeInput(t *testing.T) {
	r := &initRecorder{terminal: true, tester: fakeTester{info: "https://x (Jira 1)"}}
	stdout, stderr, err := invokeConfigInit(t, r, initToken+"\n", "--domain", "mycompany.atlassian.net", "--email", "me@example.com", "--token-stdin")
	if err != nil {
		t.Fatal(err)
	}

	if len(r.secretCalls) != 0 {
		t.Fatalf("--token-stdin must not open the masked prompt, calls: %q", r.secretCalls)
	}
	if r.savedToken != initToken {
		t.Fatalf("saved token = %q", r.savedToken)
	}
	if stderr != "" {
		t.Fatalf("stderr must stay empty, got %q", stderr)
	}
	assertTokenNeverPrinted(t, stdout, stderr)
}

func TestConfigInitTokenStdinTrimsWhitespaceAndWindowsLineEndings(t *testing.T) {
	r := &initRecorder{tester: fakeTester{info: "https://x (Jira 1)"}}
	_, _, err := invokeConfigInit(t, r, "  "+initToken+" \r\n", "--domain", "d.atlassian.net", "--email", "me@example.com", "--token-stdin")
	if err != nil {
		t.Fatal(err)
	}
	if r.savedToken != initToken {
		t.Fatalf("saved token = %q, want it trimmed", r.savedToken)
	}
}

func TestConfigInitTokenStdinNeedsDomainAndEmail(t *testing.T) {
	r := &initRecorder{}
	_, _, err := invokeConfigInit(t, r, initToken+"\n", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "--domain") || !strings.Contains(err.Error(), "--email") {
		t.Fatalf("the error must name --domain and --email, got %v", err)
	}
	assertNothingSaved(t, r)
}

func TestConfigInitTokenAndTokenStdinAreMutuallyExclusive(t *testing.T) {
	r := &initRecorder{}
	_, _, err := invokeConfigInit(t, r, initToken+"\n", "--domain", "d", "--email", "e", "--token", "x", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "--token") || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatalf("the error must name both flags, got %v", err)
	}
	assertNothingSaved(t, r)
}

func TestConfigInitRejectsExplicitEmptyTokenAlongsideStdin(t *testing.T) {
	r := &initRecorder{}
	stdout, stderr, err := invokeConfigInit(t, r, initToken+"\n", "--domain", "d", "--email", "e", "--token=", "--token-stdin")
	if err == nil || err.Error() != "--token and --token-stdin cannot be used together" {
		t.Fatalf("expected flag conflict, got %v", err)
	}
	assertNothingSaved(t, r)
	assertTokenNeverPrinted(t, stdout, stderr)
}

func TestConfigInitTokenStdinRejectsAnEmptyToken(t *testing.T) {
	r := &initRecorder{}
	_, _, err := invokeConfigInit(t, r, "  \n", "--domain", "d.atlassian.net", "--email", "me@example.com", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("an empty stdin token must be an error naming the token, got %v", err)
	}
	assertNothingSaved(t, r)
}

func TestConfigInitTokenFlagStillWorksButWarnsWithoutLeakingIt(t *testing.T) {
	r := &initRecorder{tester: fakeTester{info: "https://x (Jira 1)"}}
	stdout, stderr, err := invokeConfigInit(t, r, "", "--domain", "d.atlassian.net", "--email", "me@example.com", "--token", initToken)
	if err != nil {
		t.Fatal(err)
	}

	if r.savedToken != initToken {
		t.Fatalf("--token must keep working, saved %q", r.savedToken)
	}
	if !strings.Contains(stderr, "--token") || !strings.Contains(stderr, "--token-stdin") {
		t.Fatalf("the warning must name --token and the safer --token-stdin, got %q", stderr)
	}
	assertTokenNeverPrinted(t, stdout, stderr)
}

func TestConfigInitCancellingTheMaskedPromptSavesNothing(t *testing.T) {
	r := &initRecorder{terminal: true, secretErr: secretprompt.ErrCancelled}
	_, _, err := invokeConfigInit(t, r, "mycompany.atlassian.net\nme@example.com\n")
	if !errors.Is(err, secretprompt.ErrCancelled) {
		t.Fatalf("want a cancellation error, got %v", err)
	}
	assertNothingSaved(t, r)
}

func TestConfigInitStillRequiresEveryField(t *testing.T) {
	r := &initRecorder{}
	_, _, err := invokeConfigInit(t, r, "mycompany.atlassian.net\nme@example.com\n\n")
	if err == nil || !strings.Contains(err.Error(), "domain, email, and token are all required") {
		t.Fatalf("previous validation message changed, got %v", err)
	}
	assertNothingSaved(t, r)
}

func TestConfigInitKeepsItsConnectionReports(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		r := &initRecorder{tester: fakeTester{info: "https://x (Jira 1)"}}
		stdout, _, err := invokeConfigInit(t, r, "", "--domain", "d", "--email", "e", "--token", initToken)
		if err != nil || !strings.Contains(stdout, "✓ Configured and connected to https://x (Jira 1)") {
			t.Fatalf("err=%v stdout=%q", err, stdout)
		}
	})
	t.Run("connection fails after saving", func(t *testing.T) {
		r := &initRecorder{tester: fakeTester{err: fmt.Errorf("boom")}}
		stdout, stderr, err := invokeConfigInit(t, r, "", "--domain", "d", "--email", "e", "--token", initToken)
		if err != nil || !strings.Contains(stdout, "Config saved but connection failed: boom") {
			t.Fatalf("err=%v stdout=%q", err, stdout)
		}
		assertTokenNeverPrinted(t, stdout, stderr)
	})
	t.Run("client cannot be built after saving", func(t *testing.T) {
		r := &initRecorder{clientErr: fmt.Errorf("no client")}
		stdout, _, err := invokeConfigInit(t, r, "", "--domain", "d", "--email", "e", "--token", initToken)
		if err != nil || !strings.Contains(stdout, "Config saved but could not build client: no client") {
			t.Fatalf("err=%v stdout=%q", err, stdout)
		}
	})
}
