package tags

type probeData struct {
	Format *probeFormat `json:"format"`
}

type probeFormat struct {
	Filename       string            `json:"filename"`
	NBStreams      int               `json:"nb_streams"`
	NBPrograms     int               `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      float64           `json:"start_time,string"`
	Duration       float64           `json:"duration,string"`
	Size           string            `json:"size"`
	Bitrate        int               `json:"bit_rate,string"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}
