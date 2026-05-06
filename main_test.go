package main

import "testing"

func TestBuildQuoteSelectsLowestPricePerItem(t *testing.T) {
	store := NewStore()
	marketA := store.AddMerchant(Merchant{
		Name:     "Mercado A",
		Category: "supermercado",
		City:     "Cidade Teste",
	})
	marketB := store.AddMerchant(Merchant{
		Name:     "Mercado B",
		Category: "supermercado",
		City:     "Cidade Teste",
	})

	_, err := store.AddOffer(OfferInput{
		MerchantID: marketA.ID,
		Product:    "arroz 5kg",
		Price:      25.90,
		Unit:       "pacote",
		City:       "Cidade Teste",
		ValidUntil: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("erro ao criar oferta A: %v", err)
	}
	_, err = store.AddOffer(OfferInput{
		MerchantID: marketB.ID,
		Product:    "arroz 5kg",
		Price:      23.40,
		Unit:       "pacote",
		City:       "Cidade Teste",
		ValidUntil: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("erro ao criar oferta B: %v", err)
	}
	_, err = store.AddOffer(OfferInput{
		MerchantID: marketA.ID,
		Product:    "feijao 1kg",
		Price:      7.10,
		Unit:       "pacote",
		City:       "Cidade Teste",
		ValidUntil: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("erro ao criar oferta de feijao: %v", err)
	}

	quote := store.BuildQuote(QuoteRequest{
		City: "Cidade Teste",
		Items: []QuoteItemInput{
			{Product: "arroz 5kg", Qty: 2},
			{Product: "feijao 1kg", Qty: 1},
		},
	})

	if len(quote.Lines) != 2 {
		t.Fatalf("esperava 2 linhas, recebeu %d", len(quote.Lines))
	}
	if quote.Lines[0].BestPrice != 23.40 {
		t.Fatalf("esperava melhor preco 23.40 para arroz, recebeu %.2f", quote.Lines[0].BestPrice)
	}
	if quote.Lines[0].MerchantID != marketB.ID {
		t.Fatalf("esperava melhor lojista %s, recebeu %s", marketB.ID, quote.Lines[0].MerchantID)
	}
	if quote.GrandTotal != 53.90 {
		t.Fatalf("esperava total 53.90, recebeu %.2f", quote.GrandTotal)
	}
}

func TestBuildQuoteMarksUnavailableItems(t *testing.T) {
	store := NewStore()
	merchant := store.AddMerchant(Merchant{
		Name:     "Farmacia Central",
		Category: "farmacia",
		City:     "Cidade Teste",
	})

	_, err := store.AddOffer(OfferInput{
		MerchantID: merchant.ID,
		Product:    "dipirona 1g",
		Price:      9.99,
		Unit:       "caixa",
		City:       "Cidade Teste",
		ValidUntil: "2026-05-10",
	})
	if err != nil {
		t.Fatalf("erro ao criar oferta: %v", err)
	}

	quote := store.BuildQuote(QuoteRequest{
		City: "Cidade Teste",
		Items: []QuoteItemInput{
			{Product: "dipirona 1g", Qty: 1},
			{Product: "fralda infantil", Qty: 1},
		},
	})

	if len(quote.UnavailableItems) != 1 || quote.UnavailableItems[0] != "fralda infantil" {
		t.Fatalf("item indisponivel incorreto: %+v", quote.UnavailableItems)
	}
	if !quote.Lines[1].Unavailable {
		t.Fatalf("esperava linha 2 marcada como indisponivel")
	}
}

