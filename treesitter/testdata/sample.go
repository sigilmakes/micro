package sample

// 日本語のコメント: héllo 世界
/* block comment
   spanning three
   lines */
type Héllo struct {
	Nom  string `json:"nom,omitempty"`
	Note string
}

func Gréet(nom string) string {
	raw := `first line
second "line"
third line`
	_ = raw
	return "héllo " + nom
}

func (h *Héllo) Label() string {
	return h.Nom
}
