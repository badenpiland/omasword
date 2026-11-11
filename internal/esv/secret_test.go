package esv

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func mockKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInit() })
}

func TestKeyringSaveReplaceAndDelete(t *testing.T) {
	mockKeyring(t)
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	if err := SaveKey("test-key-not-real"); err != nil {
		t.Fatal(err)
	}
	if !hasKey("test-key-not-real") {
		t.Fatal("saved key was not readable from the keyring")
	}
	if err := SaveKey("replacement-key-not-real"); err != nil {
		t.Fatal(err)
	}
	if hasKey("test-key-not-real") || !hasKey("replacement-key-not-real") {
		t.Fatal("replace did not overwrite the keyring entry")
	}
	if err := DeleteKey(); err != nil {
		t.Fatal(err)
	}
	if Resolve("") != "" {
		t.Fatal("key still resolves after delete")
	}
	if err := DeleteKey(); err != nil {
		t.Fatal("deleting a missing key should succeed")
	}
}

func TestNoKeyringRefusesToSave(t *testing.T) {
	keyring.MockInitWithError(errors.New("org.freedesktop.secrets unavailable"))
	t.Cleanup(func() { keyring.MockInit() })
	err := SaveKey("test-key-not-real")
	if !errors.Is(err, ErrNoKeyring) {
		t.Fatalf("err = %v, want ErrNoKeyring", err)
	}
	if strings.Contains(err.Error(), "test-key-not-real") {
		t.Fatal("keyring error includes the key")
	}
	if err := DeleteKey(); !errors.Is(err, ErrNoKeyring) {
		t.Fatalf("delete err = %v, want ErrNoKeyring", err)
	}
}

func TestResolveOrderDoesNotNeedTheCommandWhenKeyringHasIt(t *testing.T) {
	mockKeyring(t)
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	if err := SaveKey("from-keyring-not-real"); err != nil {
		t.Fatal(err)
	}
	if !hasKey("from-keyring-not-real") {
		t.Fatal("keyring value was not used")
	}
	// The command is not consulted, so a failing command must not matter.
	if got := Resolve("exit 1"); got == "" || !hasKey("from-keyring-not-real") {
		t.Fatal("keyring entry was ignored")
	}
	if Resolve("exit 1") != "from-keyring-not-real" {
		t.Fatal("resolved value is not the keyring entry")
	}
}

func TestKeyCommandStdoutIsTheKeyAndIsNotAnError(t *testing.T) {
	mockKeyring(t)
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	got := Resolve("printf '%s\\nextra metadata\\n' from-cmd-not-real")
	if got != "from-cmd-not-real" {
		t.Fatal("key command should supply the first line of stdout")
	}
	// A failing command must not surface its streams. The secret is on stderr
	// here on purpose: it must not come back in an error, and Resolve has no
	// error return, so the value simply isn't used.
	if got := Resolve("printf '%s\\n' leaked-key-not-real >&2; exit 1"); got != "" {
		t.Fatal("a failing key command was treated as a key")
	}
}

func TestEnvKeyOverridesAndIsNotRequiredToExistAnywhereElse(t *testing.T) {
	mockKeyring(t)
	t.Setenv("OMASWORD_ESV_API_KEY", "from-env-not-a-real-key")
	if err := SaveKey("from-keyring-not-real"); err != nil {
		t.Fatal(err)
	}
	if Resolve("printf '%s' from-cmd-not-real") != "from-env-not-a-real-key" {
		t.Fatal("OMASWORD_ESV_API_KEY should win for this process only")
	}
}

func hasKey(want string) bool {
	return Resolve("") == want
}
