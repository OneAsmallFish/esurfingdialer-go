package state

import (
	"sync"
)

type State struct {
	mu   sync.RWMutex
	done chan struct{}

	running bool
	logged  bool

	clientID string
	algoID   string
	mac      string

	ticket string
	userIP string
	acIP   string

	schoolID string
	domain   string
	area     string

	ticketURL string
	authURL   string

	extraCfgURL map[string]string

	hostName  string
	userAgent string
}

func New() *State {
	return &State{
		running:     true,
		done:        make(chan struct{}),
		extraCfgURL: make(map[string]string),
	}
}

func (s *State) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *State) SetRunning(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 进程生命周期只允许 running -> stopped，Done 关闭后不得重新启用。
	if !s.running {
		return
	}
	if s.running && !v {
		close(s.done)
	}
	s.running = v
}

func (s *State) Done() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

func (s *State) IsLogged() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logged
}

func (s *State) SetLogged(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logged = v
}

func (s *State) ClientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clientID
}

func (s *State) SetClientID(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientID = v
}

func (s *State) AlgoID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.algoID
}

func (s *State) SetAlgoID(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.algoID = v
}

func (s *State) Mac() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mac
}

func (s *State) SetMac(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mac = v
}

func (s *State) Ticket() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ticket
}

func (s *State) SetTicket(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticket = v
}

func (s *State) UserIP() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userIP
}

func (s *State) SetUserIP(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userIP = v
}

func (s *State) AcIP() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.acIP
}

func (s *State) SetAcIP(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acIP = v
}

func (s *State) SchoolID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.schoolID
}

func (s *State) SetSchoolID(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schoolID = v
}

func (s *State) Domain() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.domain
}

func (s *State) SetDomain(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domain = v
}

func (s *State) Area() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.area
}

func (s *State) SetArea(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.area = v
}

func (s *State) TicketURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ticketURL
}

func (s *State) SetTicketURL(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticketURL = v
}

func (s *State) AuthURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authURL
}

func (s *State) SetAuthURL(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authURL = v
}

func (s *State) ExtraCfgURL() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]string, len(s.extraCfgURL))
	for k, v := range s.extraCfgURL {
		copyMap[k] = v
	}
	return copyMap
}

func (s *State) SetExtraCfgURL(v map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extraCfgURL = v
}

func (s *State) SetExtraCfgURLItem(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extraCfgURL[key] = val
}

func (s *State) ExtraCfgURLItem(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extraCfgURL[key]
}

func (s *State) HostName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hostName
}

func (s *State) SetHostName(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostName = v
}

func (s *State) UserAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userAgent
}

func (s *State) SetUserAgent(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userAgent = v
}
