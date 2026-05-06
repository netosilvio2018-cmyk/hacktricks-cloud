package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Merchant struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	City         string    `json:"city"`
	Neighborhood string    `json:"neighborhood"`
	Contact      string    `json:"contact"`
	CreatedAt    time.Time `json:"created_at"`
}

type Offer struct {
	ID           string    `json:"id"`
	MerchantID   string    `json:"merchant_id"`
	Product      string    `json:"product"`
	Price        float64   `json:"price"`
	Unit         string    `json:"unit"`
	City         string    `json:"city"`
	ValidUntil   string    `json:"valid_until"`
	CreatedAt    time.Time `json:"created_at"`
	NormalizedSK string    `json:"-"`
}

type OfferInput struct {
	MerchantID string  `json:"merchant_id"`
	Product    string  `json:"product"`
	Price      float64 `json:"price"`
	Unit       string  `json:"unit"`
	City       string  `json:"city"`
	ValidUntil string  `json:"valid_until"`
}

type QuoteItemInput struct {
	Product string  `json:"product"`
	Qty     float64 `json:"qty"`
}

type QuoteRequest struct {
	City  string           `json:"city"`
	Items []QuoteItemInput `json:"items"`
}

type QuoteLine struct {
	Product     string  `json:"product"`
	Qty         float64 `json:"qty"`
	BestPrice   float64 `json:"best_price"`
	Unit        string  `json:"unit"`
	MerchantID  string  `json:"merchant_id"`
	Merchant    string  `json:"merchant"`
	LineTotal   float64 `json:"line_total"`
	ValidUntil  string  `json:"valid_until"`
	Unavailable bool    `json:"unavailable"`
}

type QuoteResponse struct {
	City             string      `json:"city"`
	Lines            []QuoteLine `json:"lines"`
	GrandTotal       float64     `json:"grand_total"`
	UnavailableItems []string    `json:"unavailable_items"`
	GeneratedAt      time.Time   `json:"generated_at"`
}

type Store struct {
	mu           sync.RWMutex
	merchantSeq  int
	offerSeq     int
	merchants    map[string]Merchant
	offers       map[string]Offer
	offersByCity map[string][]string
}

func NewStore() *Store {
	return &Store{
		merchants:    make(map[string]Merchant),
		offers:       make(map[string]Offer),
		offersByCity: make(map[string][]string),
	}
}

func (s *Store) AddMerchant(m Merchant) Merchant {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.merchantSeq++
	m.ID = fmt.Sprintf("lojista-%03d", s.merchantSeq)
	m.CreatedAt = time.Now().UTC()
	s.merchants[m.ID] = m
	return m
}

func (s *Store) ListMerchants(city string) []Merchant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Merchant, 0, len(s.merchants))
	for _, m := range s.merchants {
		if city != "" && !strings.EqualFold(m.City, city) {
			continue
		}
		items = append(items, m)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (s *Store) AddOffer(in OfferInput) (Offer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	merchant, ok := s.merchants[in.MerchantID]
	if !ok {
		return Offer{}, fmt.Errorf("merchant_id %s não encontrado", in.MerchantID)
	}
	if !strings.EqualFold(merchant.City, in.City) {
		return Offer{}, fmt.Errorf("cidade da oferta diferente da cidade do lojista")
	}

	s.offerSeq++
	o := Offer{
		ID:           fmt.Sprintf("oferta-%04d", s.offerSeq),
		MerchantID:   in.MerchantID,
		Product:      strings.TrimSpace(in.Product),
		Price:        in.Price,
		Unit:         strings.TrimSpace(in.Unit),
		City:         strings.TrimSpace(in.City),
		ValidUntil:   strings.TrimSpace(in.ValidUntil),
		CreatedAt:    time.Now().UTC(),
		NormalizedSK: normalizeProduct(in.Product),
	}
	s.offers[o.ID] = o
	key := strings.ToLower(strings.TrimSpace(o.City))
	s.offersByCity[key] = append(s.offersByCity[key], o.ID)
	return o, nil
}

func (s *Store) ListOffers(city string) []Offer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := strings.ToLower(strings.TrimSpace(city))
	ids := s.offersByCity[key]
	resp := make([]Offer, 0, len(ids))
	for _, id := range ids {
		resp = append(resp, s.offers[id])
	}
	sort.Slice(resp, func(i, j int) bool {
		if resp[i].Product == resp[j].Product {
			return resp[i].Price < resp[j].Price
		}
		return resp[i].Product < resp[j].Product
	})
	return resp
}

func (s *Store) BuildQuote(req QuoteRequest) QuoteResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cityKey := strings.ToLower(strings.TrimSpace(req.City))
	ids := s.offersByCity[cityKey]
	offersByProduct := make(map[string][]Offer)
	for _, id := range ids {
		of := s.offers[id]
		offersByProduct[of.NormalizedSK] = append(offersByProduct[of.NormalizedSK], of)
	}
	for key := range offersByProduct {
		sort.Slice(offersByProduct[key], func(i, j int) bool {
			return offersByProduct[key][i].Price < offersByProduct[key][j].Price
		})
	}

	res := QuoteResponse{
		City:        req.City,
		Lines:       make([]QuoteLine, 0, len(req.Items)),
		GeneratedAt: time.Now().UTC(),
	}

	for _, item := range req.Items {
		norm := normalizeProduct(item.Product)
		candidates := offersByProduct[norm]
		if len(candidates) == 0 {
			res.Lines = append(res.Lines, QuoteLine{
				Product:     item.Product,
				Qty:         item.Qty,
				Unavailable: true,
			})
			res.UnavailableItems = append(res.UnavailableItems, item.Product)
			continue
		}

		best := candidates[0]
		merchant := s.merchants[best.MerchantID]
		lineTotal := best.Price * item.Qty
		res.GrandTotal += lineTotal
		res.Lines = append(res.Lines, QuoteLine{
			Product:    item.Product,
			Qty:        item.Qty,
			BestPrice:  best.Price,
			Unit:       best.Unit,
			MerchantID: best.MerchantID,
			Merchant:   merchant.Name,
			LineTotal:  lineTotal,
			ValidUntil: best.ValidUntil,
		})
	}

	return res
}

type App struct {
	store *Store
	tpl   *template.Template
}

func main() {
	store := NewStore()
	seed(store)

	tpl := template.Must(template.New("home").Parse(homePageTemplate))
	app := &App{store: store, tpl: tpl}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleHome)
	mux.HandleFunc("/api/merchants", app.handleMerchants)
	mux.HandleFunc("/api/offers", app.handleOffers)
	mux.HandleFunc("/api/quote", app.handleQuote)

	addr := ":8080"
	log.Printf("Servidor de automação local iniciado em %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("erro no servidor: %v", err)
	}
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	city := strings.TrimSpace(r.URL.Query().Get("city"))
	if city == "" {
		city = "Minha Cidade"
	}
	data := struct {
		City      string
		Merchants []Merchant
		Offers    []Offer
	}{
		City:      city,
		Merchants: a.store.ListMerchants(city),
		Offers:    a.store.ListOffers(city),
	}
	if err := a.tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleMerchants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		city := strings.TrimSpace(r.URL.Query().Get("city"))
		writeJSON(w, http.StatusOK, a.store.ListMerchants(city))
	case http.MethodPost:
		var in Merchant
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.City) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "name e city são obrigatórios",
			})
			return
		}
		if strings.TrimSpace(in.Category) == "" {
			in.Category = "geral"
		}
		saved := a.store.AddMerchant(in)
		writeJSON(w, http.StatusCreated, saved)
	default:
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleOffers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		city := strings.TrimSpace(r.URL.Query().Get("city"))
		if city == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "city é obrigatória"})
			return
		}
		writeJSON(w, http.StatusOK, a.store.ListOffers(city))
	case http.MethodPost:
		var payload struct {
			Offers []OfferInput `json:"offers"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(payload.Offers) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "envie ao menos 1 oferta"})
			return
		}
		saved := make([]Offer, 0, len(payload.Offers))
		for _, in := range payload.Offers {
			if strings.TrimSpace(in.MerchantID) == "" || strings.TrimSpace(in.Product) == "" ||
				strings.TrimSpace(in.City) == "" || in.Price <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "merchant_id, product, city e price>0 são obrigatórios",
				})
				return
			}
			if strings.TrimSpace(in.Unit) == "" {
				in.Unit = "un"
			}
			if strings.TrimSpace(in.ValidUntil) == "" {
				in.ValidUntil = time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
			}
			of, err := a.store.AddOffer(in)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			saved = append(saved, of)
		}
		writeJSON(w, http.StatusCreated, saved)
	default:
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var req QuoteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.City) == "" || len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "city e items são obrigatórios",
		})
		return
	}
	for i := range req.Items {
		if req.Items[i].Qty <= 0 {
			req.Items[i].Qty = 1
		}
	}
	resp := a.store.BuildQuote(req)
	writeJSON(w, http.StatusOK, resp)
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("json inválido: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeProduct(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "  ", " ")
	return s
}

func seed(s *Store) {
	m1 := s.AddMerchant(Merchant{
		Name:         "Super Bom Preco",
		Category:     "supermercado",
		City:         "Minha Cidade",
		Neighborhood: "Centro",
		Contact:      "+55 11 99999-0001",
	})
	m2 := s.AddMerchant(Merchant{
		Name:         "Farmacia Vida",
		Category:     "farmacia",
		City:         "Minha Cidade",
		Neighborhood: "Bairro Alto",
		Contact:      "+55 11 99999-0002",
	})
	m3 := s.AddMerchant(Merchant{
		Name:         "Atacado Popular",
		Category:     "distribuidora",
		City:         "Minha Cidade",
		Neighborhood: "Industrial",
		Contact:      "+55 11 99999-0003",
	})
	_, _ = s.AddOffer(OfferInput{
		MerchantID: m1.ID, Product: "arroz 5kg", Price: 24.90, Unit: "pacote", City: "Minha Cidade", ValidUntil: "2026-05-06",
	})
	_, _ = s.AddOffer(OfferInput{
		MerchantID: m3.ID, Product: "arroz 5kg", Price: 23.50, Unit: "pacote", City: "Minha Cidade", ValidUntil: "2026-05-06",
	})
	_, _ = s.AddOffer(OfferInput{
		MerchantID: m1.ID, Product: "oleo de soja 900ml", Price: 6.99, Unit: "garrafa", City: "Minha Cidade", ValidUntil: "2026-05-06",
	})
	_, _ = s.AddOffer(OfferInput{
		MerchantID: m2.ID, Product: "dipirona 1g", Price: 8.49, Unit: "caixa", City: "Minha Cidade", ValidUntil: "2026-05-06",
	})
}

const homePageTemplate = `
<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Automação de Ofertas da Cidade</title>
  <style>
    body { font-family: Arial, sans-serif; max-width: 1040px; margin: 24px auto; padding: 0 16px; line-height: 1.4; }
    h1, h2 { margin-bottom: 8px; }
    .card { border: 1px solid #ddd; border-radius: 10px; padding: 12px; margin-bottom: 12px; }
    table { border-collapse: collapse; width: 100%; margin-top: 8px; }
    th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
    th { background: #f4f4f4; }
    code { background: #f8f8f8; padding: 2px 6px; border-radius: 4px; }
    .small { color: #555; font-size: 14px; }
  </style>
</head>
<body>
  <h1>Central de Promoções - {{.City}}</h1>
  <p class="small">MVP para cotação automática local entre supermercados, farmácias, distribuidoras e prestadores.</p>

  <div class="card">
    <h2>Lojistas cadastrados</h2>
    <table>
      <thead>
        <tr><th>ID</th><th>Nome</th><th>Categoria</th><th>Bairro</th><th>Contato</th></tr>
      </thead>
      <tbody>
      {{range .Merchants}}
        <tr>
          <td>{{.ID}}</td>
          <td>{{.Name}}</td>
          <td>{{.Category}}</td>
          <td>{{.Neighborhood}}</td>
          <td>{{.Contact}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Ofertas ativas</h2>
    <table>
      <thead>
        <tr><th>Produto</th><th>Preço</th><th>Unidade</th><th>Lojista ID</th><th>Válido até</th></tr>
      </thead>
      <tbody>
      {{range .Offers}}
        <tr>
          <td>{{.Product}}</td>
          <td>R$ {{printf "%.2f" .Price}}</td>
          <td>{{.Unit}}</td>
          <td>{{.MerchantID}}</td>
          <td>{{.ValidUntil}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Endpoints da automação</h2>
    <ul>
      <li><code>GET /api/merchants?city=Minha Cidade</code> lista lojistas</li>
      <li><code>POST /api/merchants</code> cadastra lojista</li>
      <li><code>POST /api/offers</code> recebe promoções do lojista</li>
      <li><code>POST /api/quote</code> cota automaticamente e retorna menor preço por item</li>
    </ul>
  </div>
</body>
</html>
`
