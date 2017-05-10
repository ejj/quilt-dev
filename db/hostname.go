package db

// A Hostname is a mapping from a name to an IP.
type Hostname struct {
	ID int `json:"-"`

	Hostname, IP string
}

// HostnameSlice is an alias for []Hostname to allow for joins
type HostnameSlice []Hostname

// InsertHostname creates a new Hostname row and inserts it into 'db'.
func (db Database) InsertHostname() Hostname {
	result := Hostname{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromHostname gets all hostnames in the database that satisfy 'check'.
func (db Database) SelectFromHostname(check func(Hostname) bool) []Hostname {
	var result []Hostname
	for _, row := range db.selectRows(HostnameTable) {
		if check == nil || check(row.(Hostname)) {
			result = append(result, row.(Hostname))
		}
	}
	return result
}

// SelectFromHostname gets all hostnames in the database that satisfy the 'check'.
func (conn Conn) SelectFromHostname(check func(Hostname) bool) []Hostname {
	var hostnames []Hostname
	conn.Txn(HostnameTable).Run(func(view Database) error {
		hostnames = view.SelectFromHostname(check)
		return nil
	})
	return hostnames
}

func (r Hostname) getID() int {
	return r.ID
}

func (r Hostname) String() string {
	return defaultString(r)
}

func (r Hostname) less(row row) bool {
	r2 := row.(Hostname)

	switch {
	case r.Hostname != r2.Hostname:
		return r.Hostname < r2.Hostname
	default:
		return r.ID < r2.ID
	}
}

// Get returns the value contained at the given index
func (hs HostnameSlice) Get(i int) interface{} {
	return hs[i]
}

// Len returns the number of items in the slice
func (hs HostnameSlice) Len() int {
	return len(hs)
}

// Less implements less than for sort.Interface.
func (hs HostnameSlice) Less(i, j int) bool {
	return hs[i].less(hs[j])
}

// Swap implements swapping for sort.Interface.
func (hs HostnameSlice) Swap(i, j int) {
	hs[i], hs[j] = hs[j], hs[i]
}
