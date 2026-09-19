package domain

// Partner — магазин, куда ведут ссылки «где купить» (kind goods: техника с маркетплейса)
// и «собрать корзину» (kind grocery: доставка продуктов). URL — шаблон с {q}.
// Affiliate — ссылка партнёрская: интерфейс подписывает её «Реклама» и показывает erid (закон о рекламе РФ).
type Partner struct {
	Code      string `json:"code"`
	Country   string `json:"country"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Affiliate bool   `json:"affiliate"`
	Erid      string `json:"erid"`
	Active    bool   `json:"active"`
	Priority  int    `json:"priority"`
}

// APIKey — ключ доступа к API от имени пользователя (админа). Key заполняется только в ответе на создание.
type APIKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	Key        string `json:"key,omitempty"`
}
