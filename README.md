# Automacao local de ofertas da cidade

MVP em Go para criar uma central de promocoes locais (supermercados, farmacias,
distribuidoras e prestadores), onde:

- o lojista cadastra promocoes do dia
- o cliente pede uma cotacao por lista de itens
- a plataforma retorna automaticamente o menor preco por item na cidade

## Stack

- Go 1.22 (somente biblioteca padrao)
- API HTTP + painel web simples no endpoint raiz
- armazenamento em memoria (MVP)

## Como rodar

```bash
go run .
```

Servidor inicia em `http://localhost:8080`.

## Fluxo principal

1. Cadastrar lojista (`POST /api/merchants`)
2. Lojista enviar ofertas (`POST /api/offers`)
3. Cliente solicitar cotacao (`POST /api/quote`)
4. API devolve menor preco por item com total consolidado

## Endpoints

### Listar lojistas por cidade

```bash
curl -s "http://localhost:8080/api/merchants?city=Minha%20Cidade"
```

### Cadastrar lojista

```bash
curl -s -X POST http://localhost:8080/api/merchants \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Mercado do Bairro",
    "category":"supermercado",
    "city":"Minha Cidade",
    "neighborhood":"Centro",
    "contact":"+55 11 99999-9999"
  }'
```

### Enviar promocoes do dia

```bash
curl -s -X POST http://localhost:8080/api/offers \
  -H 'Content-Type: application/json' \
  -d '{
    "offers":[
      {
        "merchant_id":"lojista-001",
        "product":"arroz 5kg",
        "price":24.90,
        "unit":"pacote",
        "city":"Minha Cidade",
        "valid_until":"2026-05-06"
      },
      {
        "merchant_id":"lojista-001",
        "product":"oleo de soja 900ml",
        "price":6.99,
        "unit":"garrafa",
        "city":"Minha Cidade",
        "valid_until":"2026-05-06"
      }
    ]
  }'
```

### Cotacao automatica (menor preco)

```bash
curl -s -X POST http://localhost:8080/api/quote \
  -H 'Content-Type: application/json' \
  -d '{
    "city":"Minha Cidade",
    "items":[
      {"product":"arroz 5kg","qty":2},
      {"product":"oleo de soja 900ml","qty":1},
      {"product":"dipirona 1g","qty":1}
    ]
  }'
```

## Testes

Executar:

```bash
go test ./...
```

Cobertura atual valida:

- escolha do menor preco por item entre lojas
- calculo do total consolidado
- identificacao de itens indisponiveis

## Proximos passos recomendados

- persistencia em PostgreSQL
- autenticacao por perfil (lojista/admin/cliente)
- normalizacao inteligente de produtos (sinonimos, marcas e unidades)
- integracao com WhatsApp para consulta automatica do cliente
