package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
	"github.com/eulerbutcooler/surang/internal/client"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))
	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)
	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)
)

type apiClient struct {
	base   string
	client *http.Client
}

func newAPIClient(base string) apiClient {
	return apiClient{base: base, client: &http.Client{Timeout: 15 * time.Second}}
}

type apiError struct {
	status int
	msg    string
}

func (e apiError) Error() string { return e.msg }

func (c apiClient) post(path string, body any, bearer string) (map[string]string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	base := c.base
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("api error: %s", resp.Status)
		if m := out["error"]; m != "" {
			msg = m
		}
		return nil, apiError{status: resp.StatusCode, msg: msg}
	}
	return out, nil
}

type mintedMsg struct{ apiToken string }
type mintErrMsg struct{ err error }

func mintCmd(base, email, password, expiry, mode string) tea.Cmd {
	return func() tea.Msg {
		base = strings.TrimSpace(base)
		c := newAPIClient(base)
		if mode == "signup" {
			_, err := c.post("/api/signup", map[string]string{
				"email": email, "password": password,
			}, "")
			if err != nil {
				var ae apiError
				if !errors.As(err, &ae) || ae.status != http.StatusConflict {
					return mintErrMsg{err}
				}
			}
		}

		out, err := c.post("/api/login", map[string]string{
			"email": email, "password": password,
		}, "")
		if err != nil {
			return mintErrMsg{err}
		}
		sess := out["session_token"]

		out, err = c.post("/api/tokens", map[string]string{"expires": expiry}, sess)
		if err != nil {
			return mintErrMsg{err}
		}
		apiToken := out["api_token"]

		if err := client.SaveConfig(client.Config{API: base, Token: apiToken}); err != nil {
			return mintErrMsg{err}
		}
		return mintedMsg{apiToken: apiToken}
	}
}

type step int

const (
	stepForm step = iota
	stepWorking
	stepDone
	stepError
)

type loginModel struct {
	mode     string
	step     step
	form     *huh.Form
	spinner  spinner.Model
	server   string
	email    string
	password string
	expiry   string
	err      error
}

func newLoginModel() *loginModel {
	m := &loginModel{
		spinner: spinner.New(),
		mode:    "login",
		server:  "http://localhost:9000"}
	m.form = makeLoginForm(m)
	return m
}

func makeLoginForm(m *loginModel) *huh.Form {

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Account").
				Options(
					huh.NewOption("I have an account - log in", "login"),
					huh.NewOption("I'm new - create an account", "signup"),
				).
				Value(&m.mode),
			huh.NewInput().
				Title("Server").
				Description("surang API address").
				Placeholder("localhost:9000").
				Value(&m.server),
			huh.NewInput().
				Title("Email").
				Value(&m.email).
				Validate(func(s string) error {
					if s == "" || !strings.Contains(s, "@") {
						return fmt.Errorf(`¯\_(ツ)_/¯ - enter a valid email`)
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&m.password).
				Validate(func(s string) error {
					if len(s) < 6 {
						return fmt.Errorf(`¯\_(ツ)_/¯ - min 6 characters`)
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Token expiry").
				Options(
					huh.NewOption("1 hour", "1h"),
					huh.NewOption("1 day", "1d"),
					huh.NewOption("1 week", "1w"),
					huh.NewOption("never", "never"),
				).
				Value(&m.expiry),
		),
	).WithShowHelp(true)
}

func (m *loginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.step == stepError {
				m.step = stepForm
				m.form = makeLoginForm(m)
				return m, m.form.Init()
			}
		}
	case spinner.TickMsg:
		if m.step == stepWorking {
			sp, cmd := m.spinner.Update(msg)
			m.spinner = sp
			return m, cmd
		}
	case mintedMsg:
		m.step = stepDone
		return m, tea.Quit
	case mintErrMsg:
		m.err = msg.err
		m.step = stepError
		return m, nil
	}

	if m.step == stepForm {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}
		if m.form.State == huh.StateCompleted {
			m.step = stepWorking
			return m, tea.Batch(mintCmd(m.server, m.email, m.password, m.expiry, m.mode), m.spinner.Tick)
		}
		return m, cmd
	}
	return m, nil
}

func (m *loginModel) View() string {
	switch m.step {
	case stepWorking:
		return titleStyle.Render("surang") + "\n\n  " + m.spinner.View() + " minting token..."
	case stepDone:
		return okStyle.Render("(─‿‿─) token saved to ~/.surang/config.json") +
			"\n\n  run: surang-client -tunnel web=localhost:3000"
	case stepError:
		return errStyle.Render("(⌣́_⌣̀) "+m.err.Error()) +
			"\n\n enter: try again · ctrl+c: quit"
	}
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render(figure.NewFigure("SURANG", "graffiti", true).String())
	return banner + "\n" + m.form.View()
}

func runLogin() error {
	p := tea.NewProgram(newLoginModel())
	_, err := p.Run()
	return err
}
