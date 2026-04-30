# Rinha de Backend 2026 - Detecção de Fraude em C

Backend em **C** para a **Rinha de Backend 2026**, construído com foco em **baixa latência**, **alto throughput**, **precisão determinística no top-5** e uso econômico de CPU/memória para o endpoint de detecção de fraude.

A aplicação recebe uma transação em `POST /fraud-score`, transforma o JSON em um vetor numérico de **14 dimensões**, consulta um índice vetorial **IVF/K-Means** carregado em memória, recupera os **5 vizinhos mais próximos** e retorna uma decisão simples:

```json
{"approved":true,"fraud_score":0.4000}
```

A decisão não vem de regras fixas isoladas. Ela é calculada por similaridade contra um dataset de referência já vetorizado e indexado. Em outras palavras: o programa compara a transação recebida com exemplos anteriores e decide pelo comportamento dos vizinhos mais próximos.

---

## Resumo da solução

Este backend implementa cinco ideias principais:

1. **Servidor HTTP manual em C com `io_uring`**  
   O servidor não usa framework web. Ele abre socket, aceita conexões, lê requisições e escreve respostas usando `io_uring`.

2. **Transporte por Unix Domain Socket ou TCP**  
   Por padrão a aplicação escuta em **Unix Domain Socket**, ideal para comunicação local com HAProxy dentro do mesmo host/container. Também pode escutar em TCP se `LISTEN_TCP=1`.

3. **Busca vetorial IVF6**  
   O índice é pré-processado em um arquivo binário `index.bin`. Em runtime, o backend carrega centroids, bounding boxes dos clusters, offsets, vetores quantizados, labels e ids originais. Na requisição, só faz busca.

4. **Top-5 determinístico**  
   O top-5 é ordenado por distância e, em caso de empate, pela posição original do vetor no `references.json.gz`. Isso evita divergência de resultado quando há empates ou distâncias muito próximas.

5. **Resposta pré-montada**  
   Como o resultado final só pode ter 6 valores possíveis de score (`0.0000`, `0.2000`, `0.4000`, `0.6000`, `0.8000`, `1.0000`), as respostas HTTP são montadas uma vez no startup e reutilizadas.

---

## Arquitetura geral

A arquitetura esperada para o desafio usa um balanceador na frente e duas instâncias da API atrás.

```text
                   ┌──────────────────────┐
                   │      k6 / tester      │
                   │  carga do desafio     │
                   └───────────┬──────────┘
                               │ HTTP :9999
                               ▼
                   ┌──────────────────────┐
                   │        HAProxy        │
                   │   balanceia requests  │
                   └───────┬────────┬─────┘
                           │        │
              Unix Socket  │        │  Unix Socket
                           │        │
                           ▼        ▼
             ┌────────────────┐  ┌────────────────┐
             │     API 1      │  │     API 2      │
             │ C + io_uring   │  │ C + io_uring   │
             │ IVF6 Search    │  │ IVF6 Search    │
             │ /sockets/api1  │  │ /sockets/api2  │
             └───────┬────────┘  └───────┬────────┘
                     │                   │
                     │ heap/load          │ heap/load
                     ▼                   ▼
             ┌────────────────┐  ┌────────────────┐
             │  index.bin     │  │  index.bin     │
             │  IVF6 format   │  │  IVF6 format   │
             └────────────────┘  └────────────────┘
```

No código, cada processo da API carrega o índice na inicialização e mantém tudo em memória. A partir daí, o caminho quente da requisição evita I/O em disco.

---

## Fluxo completo de uma requisição

```text
POST /fraud-score
       │
       ▼
┌──────────────────────┐
│ io_uring recebe bytes │
│ OP_ACCEPT / OP_READ   │
└──────────┬───────────┘
           │
           ▼
┌─────────────────────────────┐
│ Parser HTTP mínimo           │
│ - encontra \r\n\r\n          │
│ - lê Content-Length          │
│ - separa o body JSON         │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ vectorizer_build()           │
│ JSON -> vetor float[14]      │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ ivf_search_fraud_votes()     │
│ - quantiza query para int16  │
│ - escolhe clusters próximos  │
│ - escaneia candidatos IVF    │
│ - repara por bounding-box    │
│ - mantém top 5 vizinhos      │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ Conta labels fraud no top 5  │
│ frauds = 0..5                │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ Resposta pré-montada         │
│ score = frauds * 0.2         │
│ approved = frauds < 3        │
└──────────┬──────────────────┘
           │
           ▼
HTTP/1.1 200 OK
{"approved":true,"fraud_score":0.4000}
```

---

## Endpoints

### `GET /ready`

Endpoint de prontidão.

Resposta:

```http
HTTP/1.1 200 OK
Content-Length: 0
Connection: keep-alive
```

Ele é útil para healthcheck do balanceador ou do orquestrador. No código, essa resposta já fica pré-montada em `http_responses_init()`.

---

### `POST /fraud-score`

Endpoint principal de classificação de fraude.

O body esperado é um JSON contendo pelo menos os objetos:

- `transaction`
- `customer`
- `merchant`
- `terminal`

Opcionalmente pode conter `last_transaction`.

Exemplo de payload:

```json
{
  "transaction": {
    "id": "tx-1788243118",
    "amount": 1250.75,
    "installments": 3,
    "requested_at": "2026-01-15T22:31:10Z"
  },
  "customer": {
    "avg_amount": 300.00,
    "tx_count_24h": 4,
    "known_merchants": ["merchant-001", "merchant-002"]
  },
  "merchant": {
    "id": "merchant-999",
    "mcc": "7995",
    "avg_amount": 700.00
  },
  "terminal": {
    "is_online": true,
    "card_present": false,
    "km_from_home": 850.0
  },
  "last_transaction": {
    "timestamp": "2026-01-15T21:50:10Z",
    "km_from_current": 25.0
  }
}
```

Resposta possível:

```json
{"approved":false,"fraud_score":0.6000}
```

---

## Regra de aprovação

A busca retorna quantos dos **5 vizinhos mais próximos** têm label de fraude.

```text
top 5 vizinhos: [legit, fraud, legit, fraud, legit]
frauds = 2
fraud_score = 2 * 0.2 = 0.4
approved = true
```

A regra implementada é:

```c
score = frauds * 0.2f;
approved = frauds < 3;
```

Ou seja:

| Votos de fraude no top-5 | `fraud_score` | `approved` |
|---:|---:|:---|
| 0 | 0.0000 | `true` |
| 1 | 0.2000 | `true` |
| 2 | 0.4000 | `true` |
| 3 | 0.6000 | `false` |
| 4 | 0.8000 | `false` |
| 5 | 1.0000 | `false` |

A transação é aprovada enquanto menos da metade do top-5 for fraude. A partir de 3 votos de fraude, ela é negada.

---

## Como o vetor de 14 dimensões é montado

O arquivo `vectorizer.c` transforma o JSON da transação em um vetor `float q[14]`. Cada posição representa uma característica normalizada da transação.

```text
JSON da transação
       │
       ▼
┌─────────────────────────────┐
│ Campos financeiros           │ amount, installments, médias
├─────────────────────────────┤
│ Campos temporais             │ hora, dia da semana, última tx
├─────────────────────────────┤
│ Campos geográficos           │ km de casa, km da última tx
├─────────────────────────────┤
│ Comportamento do cliente     │ tx_count_24h, merchant conhecido
├─────────────────────────────┤
│ Terminal/canal               │ online, cartão presente
├─────────────────────────────┤
│ Risco do MCC                 │ tabela mcc_risk.json/default
└─────────────────────────────┘
       │
       ▼
float q[14]
```

Dimensões usadas:

| Índice | Feature | Origem | Normalização / regra |
|---:|---|---|---|
| `q[0]` | Valor da transação | `transaction.amount` | `amount / AMOUNT_DIVISOR` |
| `q[1]` | Parcelas | `transaction.installments` | `installments / INSTALLMENTS_DIVISOR` |
| `q[2]` | Valor vs média do cliente | `amount / customer.avg_amount` | `(amount / avg) / 10` |
| `q[3]` | Hora do dia | `transaction.requested_at` | `hour / 23` |
| `q[4]` | Dia da semana | `transaction.requested_at` | `weekday / 6`, segunda = 0 |
| `q[5]` | Minutos desde última transação | `last_transaction.timestamp` | `minutes / 1440`; se ausente, `-1.0` |
| `q[6]` | Distância da última transação | `last_transaction.km_from_current` | `km / KM_DIVISOR`; se ausente, `-1.0` |
| `q[7]` | Distância de casa | `terminal.km_from_home` | `km_from_home / KM_DIVISOR` |
| `q[8]` | Transações em 24h | `customer.tx_count_24h` | `tx_count_24h / TX24H_DIVISOR` |
| `q[9]` | Compra online | `terminal.is_online` | `1.0` ou `0.0` |
| `q[10]` | Cartão presente | `terminal.card_present` | `1.0` ou `0.0` |
| `q[11]` | Merchant desconhecido | `customer.known_merchants` | `1.0` se desconhecido, senão `0.0` |
| `q[12]` | Risco do MCC | `merchant.mcc` | `mcc_risk_get(mcc)` |
| `q[13]` | Média do merchant | `merchant.avg_amount` | `avg / MERCHANT_AMOUNT_DIVISOR` |

Todas as features normalizadas passam por `clamp01()`, ou seja, ficam no intervalo de `0.0` a `1.0`, exceto os campos `q[5]` e `q[6]` quando `last_transaction` está ausente. Nesse caso, eles ficam como `-1.0` para preservar o mesmo padrão do dataset de referência.

---

## MCC risk

O risco do MCC é carregado por `mcc_risk_load()`.

O código tenta ler o arquivo configurado em:

```bash
MCC_RISK_PATH=resources/mcc_risk.json
```

Se o arquivo não existir, usa uma tabela default embutida no binário.

Tabela default atual:

| MCC | Risco |
|---:|---:|
| 5411 | 0.15 |
| 5812 | 0.30 |
| 5912 | 0.20 |
| 5944 | 0.45 |
| 7801 | 0.80 |
| 7802 | 0.75 |
| 7995 | 0.85 |
| 4511 | 0.35 |
| 5311 | 0.25 |
| 5999 | 0.50 |

MCC desconhecido também cai em `0.50`.

Exemplo de `mcc_risk.json`:

```json
{
  "5411": 0.15,
  "7995": 0.85,
  "5999": 0.50
}
```

---

## Busca vetorial IVF6

A sigla IVF vem de **Inverted File Index**. A ideia é dividir o espaço vetorial em clusters. Em vez de comparar a transação nova com todos os vetores do dataset logo de início, o programa:

1. mede a distância da query até todos os centroids;
2. escolhe os `N` clusters mais próximos;
3. escaneia os vetores desses clusters;
4. mantém os 5 melhores vizinhos encontrados;
5. usa bounding boxes para reparar a busca e visitar clusters que ainda podem conter um vizinho melhor ou empatado.

```text
                          Espaço vetorial 14D

                    cluster 12              cluster 88
                 ┌──────────────┐        ┌──────────────┐
                 │  x x x x x   │        │  x x x x x   │
                 │ x x  C12 x   │        │ x  C88  x x  │
                 └──────────────┘        └──────────────┘

        query q ───────► calcula distância até todos os centroids

                 cluster 201             cluster 7
                 ┌──────────────┐        ┌──────────────┐
                 │  x x C201 x  │        │ x x C7 x x   │
                 │   x x x x    │        │  x x x x x   │
                 └──────────────┘        └──────────────┘

Depois disso, a busca varre os clusters escolhidos por `IVF_NPROBE`
e repara a busca com base nas bounding boxes dos demais clusters.
```

No código atual:

- `IVF_CLUSTERS = 256`
- `IVF_DEFAULT_NPROBE = 32`
- `IVF_MAX_NPROBE = 512`
- `K_NEIGHBORS = 5`
- `DIM = 14`
- `FIX_SCALE = 10000.0f`

O parâmetro mais importante em runtime é:

```bash
IVF_NPROBE=1
```

No `docker-compose.yml` atual, `IVF_NPROBE` está configurado como `1`. No `Dockerfile`, o default da imagem está como `20`. Sem variável de ambiente, o código usa `IVF_DEFAULT_NPROBE`, que é `32`.

Quanto maior o `IVF_NPROBE`, mais clusters são consultados inicialmente:

```text
IVF_NPROBE baixo
  + rápido no scan inicial
  - depende mais do repair por bounding-box

IVF_NPROBE alto
  + top-5 inicial tende a vir melhor
  - consome mais CPU antes do repair
```

Também existe:

```bash
CANDIDATES=0
```

Quando `CANDIDATES=0`, não há limite artificial de candidatos no scan inicial e o repair por bounding-box fica ativo. Quando `CANDIDATES` é maior que zero, a busca limita a quantidade de candidatos escaneados e o repair por bounding-box não é executado.

---

## Bounding-box repair

Cada cluster possui uma bounding box quantizada:

```text
bbox_min[cluster][dim]
bbox_max[cluster][dim]
```

Ela representa o menor e o maior valor de cada dimensão dentro daquele cluster.

Depois do scan inicial dos clusters escolhidos por `IVF_NPROBE`, o código calcula a menor distância possível entre a query e a bounding box de cada cluster ainda não visitado:

```text
se query está dentro do intervalo da dimensão -> contribuição 0
se query está abaixo do mínimo              -> distância até o mínimo
se query está acima do máximo               -> distância até o máximo
```

Se essa distância mínima possível for menor ou igual ao pior item atual do top-5, aquele cluster ainda pode conter um vetor melhor ou empatado. Nesse caso, o cluster é escaneado.

```text
Top-5 atual possui pior distância = W

cluster não visitado
        │
        ▼
calcula bbox_lower_bound(query, cluster)
        │
        ├── se lower_bound > W
        │       ignora cluster
        │
        └── se lower_bound <= W
                escaneia cluster
```

Esse repair aumenta a precisão do IVF porque reduz a chance de perder um vizinho real que ficou fora dos clusters escolhidos inicialmente.

---

## Top-5 vizinhos

Durante o scan, o código mantém apenas os 5 melhores candidatos.

```text
candidato novo
      │
      ▼
calcula distância ao vetor da query
      │
      ▼
┌──────────────────────────────────────┐
│ (dist, original_id) melhor que o pior?│
└──────────────────┬───────────────────┘
                   │ sim
                   ▼
           substitui o pior
                   │
                   ▼
           recalcula quem é o pior
```

O top-5 usa desempate determinístico:

```text
1. menor distância
2. se empatar, menor original_id
```

`original_id` é a posição original do vetor no `references.json.gz`, antes da ordenação por cluster. Isso é importante porque o índice reorganiza os vetores por cluster, mas a ordem original precisa continuar disponível para desempates estáveis.

O programa evita ordenar todos os candidatos. Ele só mantém um conjunto fixo:

```c
uint64_t best_d[5];
uint8_t  best_l[5];
uint32_t best_id[5];
```

No final:

```c
return (best_l[0] == 1) +
       (best_l[1] == 1) +
       (best_l[2] == 1) +
       (best_l[3] == 1) +
       (best_l[4] == 1);
```

Esse retorno é a quantidade de vizinhos fraudulentos entre os 5 mais próximos.

---

## Quantização para `int16`

O índice não armazena os vetores como `float` em runtime. Ele usa `int16_t`, reduzindo memória e acelerando cálculo.

A escala atual é:

```c
#define FIX_SCALE 10000.0f
```

Isso foi escolhido para bater com a grade decimal do `references.json.gz`, que usa valores como `0.0833`, `0.8261`, `0.0416`, `-1`, `0` e `1`.

```text
float normalizado       int16 quantizado
0.0000              ->      0
0.5000              ->   5000
1.0000              ->  10000
-1.0000             -> -10000
0.0833              ->    833
0.8261              ->   8261
```

A query recebida na requisição também é quantizada antes da busca:

```c
for (int j = 0; j < DIM; j++) {
    q[j] = quantize_fixed(q_float[j]);
}
```

A distância é calculada em inteiro e acumulada em `uint64_t`:

```text
int16 query/vector
      │
      ▼
int32 diff
      │
      ▼
diff * diff
      │
      ▼
uint64 distance
```

Isso evita erro de `float` no top-5 e evita overflow. Com escala `10000`, a diferença máxima por dimensão pode chegar a `20000`, e `20000² * 14` passa de `uint32_t`, por isso a distância usa 64 bits.

---

## Layout de memória: SoA em vez de AoS

Um ponto importante do código é que os vetores são armazenados por dimensão, não por registro.

Em vez disso:

```text
vetor 0: [d0, d1, d2, ..., d13]
vetor 1: [d0, d1, d2, ..., d13]
vetor 2: [d0, d1, d2, ..., d13]
```

O backend usa isso:

```text
dim[0]:  d0_v0,  d0_v1,  d0_v2,  d0_v3, ...
dim[1]:  d1_v0,  d1_v1,  d1_v2,  d1_v3, ...
dim[2]:  d2_v0,  d2_v1,  d2_v2,  d2_v3, ...
...
dim[13]: d13_v0, d13_v1, d13_v2, d13_v3, ...
```

Esse layout é chamado de **Structure of Arrays**. Ele ajuda o scan escalar e principalmente o caminho AVX2, porque a mesma dimensão de vários vetores fica contígua na memória.

Além das dimensões, o índice também mantém arrays separados para:

```text
labels[n]
orig_ids[n]
centroids[IVF_CLUSTERS][DIM]
bbox_min[IVF_CLUSTERS][DIM]
bbox_max[IVF_CLUSTERS][DIM]
cluster_start[IVF_CLUSTERS]
cluster_end[IVF_CLUSTERS]
```

---

## Caminho escalar e caminho AVX2

O arquivo `ivf_search.c` possui dois caminhos de execução:

- `scan_range_scalar()`
- `scan_range_avx2()` quando o binário é compilado com `__AVX2__`

No startup, o programa imprime:

```text
engine: IVF/kmeans + int16 + top5 seco + AVX2
```

ou:

```text
engine: IVF/kmeans + int16 + top5 seco + escalar
```

O caminho AVX2 calcula 8 distâncias em paralelo usando registradores de 256 bits. Diferente da versão antiga, o caminho AVX2 atual não converte os vetores para `float`; ele calcula a distância com inteiros e acumula em 64 bits.

```text
AVX2 scan inteiro

query dimensão 0 ─────┐
query dimensão 1 ─────┤
query dimensão 2 ─────┤
...                   ├──► calcula distância para 8 vetores por vez
query dimensão 13 ────┘

lanes:      0    1    2    3    4    5    6    7
vetores:   v0   v1   v2   v3   v4   v5   v6   v7
```

O cálculo usa intrinsics como:

```c
_mm256_cvtepi16_epi32
_mm256_sub_epi32
_mm256_mullo_epi32
_mm256_cvtepi32_epi64
_mm256_add_epi64
```

---

## Índice binário `IVF6`

O índice carregado por `dataset_load_index()` tem magic header `IVF6`.

Estrutura lógica:

```text
┌──────────────────────────────┐
│ magic = "IVF6"               │
├──────────────────────────────┤
│ uint32_t n                   │ número de vetores
├──────────────────────────────┤
│ uint32_t k                   │ número de clusters, esperado IVF_CLUSTERS
├──────────────────────────────┤
│ uint32_t d                   │ dimensões, esperado 14
├──────────────────────────────┤
│ uint32_t stride              │ esperado 14
├──────────────────────────────┤
│ float scale                  │ esperado 10000.0
├──────────────────────────────┤
│ float centroids[k][d]        │ centroids do K-Means
├──────────────────────────────┤
│ int16_t bbox_min[k][d]       │ mínimo por dimensão em cada cluster
├──────────────────────────────┤
│ int16_t bbox_max[k][d]       │ máximo por dimensão em cada cluster
├──────────────────────────────┤
│ uint32_t offsets[k + 1]      │ início/fim de cada cluster
├──────────────────────────────┤
│ int16_t vectors[n][d]        │ vetores quantizados
├──────────────────────────────┤
│ uint8_t labels[n]            │ 0 legit, 1 fraud
├──────────────────────────────┤
│ uint32_t orig_ids[n]         │ posição original no references.json.gz
└──────────────────────────────┘
```

Ao carregar, o código valida:

```text
magic == IVF6
k == IVF_CLUSTERS
d == DIM
stride == DIM
scale == FIX_SCALE
```

Depois converte os vetores para o layout SoA em memória:

```text
arquivo: vectors[n][14]
memória: dim[14][n]
```

---

## Como gerar o índice

O projeto inclui `build_index.c`, um programa auxiliar para transformar um arquivo de referências em `index.bin`.

Uso depois de compilar:

```bash
./build/build_index resources/references.json.gz resources/index.bin
```

Ou, se o arquivo não estiver compactado:

```bash
./build/build_index resources/references.json resources/index.bin
```

O gerador:

1. lê objetos contendo `vector` e `label`;
2. carrega os vetores em `float`;
3. treina K-Means;
4. atribui cada vetor ao cluster mais próximo;
5. ordena os vetores por cluster;
6. quantiza os vetores para `int16` usando `FIX_SCALE=10000`;
7. calcula `bbox_min` e `bbox_max` para cada cluster;
8. preserva o `original_id` de cada vetor;
9. grava o arquivo binário `IVF6`.

Fluxo:

```text
references.json.gz
       │
       ▼
parse de vector + label
       │
       ▼
vetores float[14]
       │
       ▼
treino K-Means, K=IVF_K
       │
       ▼
atribuição vetor -> cluster
       │
       ▼
ordenação por cluster
       │
       ▼
quantização int16 scale=10000
       │
       ▼
bounding boxes + original ids
       │
       ▼
resources/index.bin
```

Variáveis úteis para geração:

| Variável | Default | Descrição |
|---|---:|---|
| `IVF_TRAIN_SAMPLE` | `131072` | Quantos vetores usar na amostra de treino do K-Means |
| `IVF_TRAIN_ITERS` | `10` | Número de iterações de K-Means |

Exemplo:

```bash
IVF_TRAIN_SAMPLE=262144 IVF_TRAIN_ITERS=15 \
  ./build/build_index resources/references.json.gz resources/index.bin
```

Importante: `IVF_K` em `build_index.c` precisa bater com `IVF_CLUSTERS` em `common.h`. Se mudar a quantidade de clusters, mude nos dois lugares e recrie o `index.bin`.

---

## Servidor HTTP com `io_uring`

O arquivo `iouring_server.c` implementa um servidor HTTP simples com três operações:

```c
#define OP_ACCEPT 1
#define OP_READ   2
#define OP_WRITE  3
```

O loop principal funciona assim:

```text
┌──────────────────────────────┐
│ io_uring_wait_cqe             │ espera conclusão
└──────────────┬───────────────┘
               │
               ▼
        identifica operação
               │
   ┌───────────┼───────────┐
   │           │           │
   ▼           ▼           ▼
ACCEPT        READ        WRITE
   │           │           │
   ▼           ▼           ▼
cria conn   processa   envia resposta
nova read   request    ou continua write
```

Cada conexão é representada por:

```c
typedef struct {
    int fd;
    int used;
    int close_after_write;
    char req_buf[REQ_BUF_SIZE];
    size_t req_len;
    char res_buf[RES_BUF_SIZE];
    size_t res_len;
    size_t res_sent;
} conn_t;
```

O servidor suporta keep-alive para respostas bem-sucedidas. Em erros como payload inválido, payload grande demais ou rota inexistente, fecha a conexão após escrever a resposta.

---

## Unix Domain Socket e TCP

Por padrão, o backend sobe em Unix Domain Socket:

```bash
UDS_PATH=/tmp/rinha.sock
```

Isso evita overhead de TCP quando o balanceador está no mesmo ambiente.

```text
HAProxy ──► /tmp/rinha.sock ──► API C
```

Também é possível usar TCP:

```bash
LISTEN_TCP=1 PORT=9999 HOST=0.0.0.0
```

```text
HAProxy ──► 127.0.0.1:9999 ──► API C
```

O código decide o transporte aqui:

```c
static int create_server_socket(void) {
    return g_cfg.use_tcp ? create_tcp_socket() : create_uds_socket();
}
```

---

## Multiprocessamento com `fork()`

O `main.c` permite subir múltiplos workers:

```bash
WORKERS=2
```

O processo principal carrega configuração, respostas, MCC risk e índice. Depois cria workers com `fork()`.

```text
main
 │
 ├── carrega config
 ├── prepara respostas HTTP
 ├── carrega mcc_risk
 ├── carrega index.bin IVF6
 │
 ├── fork worker 1
 ├── fork worker 2
 └── server_run_forever()
```

Como o índice é carregado antes do `fork()`, o sistema operacional pode compartilhar páginas de memória entre processos por copy-on-write, desde que essas páginas não sejam modificadas.

---

## Variáveis de ambiente

### Arquivos e índice

| Variável | Default | Descrição |
|---|---|---|
| `INDEX_PATH` | `resources/index.bin` | Caminho do índice IVF6 |
| `MCC_RISK_PATH` | `resources/mcc_risk.json` | Caminho da tabela de risco MCC |

### Busca IVF

| Variável | Default no código | Default no Dockerfile | Valor no compose | Mínimo | Máximo | Descrição |
|---|---:|---:|---:|---:|---:|---|
| `IVF_NPROBE` | `32` | `20` | `1` | `1` | `512` | Quantos clusters IVF consultar inicialmente |
| `CANDIDATES` | `0` | `0` | `0` | `0` | `2000000` | Limite máximo de candidatos escaneados. `0` desativa o limite e mantém o repair por bounding-box ativo |

### Workers

| Variável | Default | Mínimo | Máximo | Descrição |
|---|---:|---:|---:|---|
| `WORKERS` | `1` | `1` | `16` | Quantidade de processos workers |

### Transporte

| Variável | Default | Descrição |
|---|---|---|
| `LISTEN_TCP` | `0` | `0` usa Unix Domain Socket; `1` usa TCP |
| `PORT` | `9999` | Porta TCP |
| `HOST` | `0.0.0.0` | Host TCP |
| `UDS_PATH` | `/tmp/rinha.sock` | Caminho do Unix Domain Socket |
| `SOCKET_PATH` | `/tmp/rinha.sock` | Alias usado se `UDS_PATH` não estiver definido |
| `UDS_MODE` | `666` | Permissão do socket |
| `UNLINK_UDS` | `1` | Remove socket antigo antes do bind |
| `TCP_NODELAY` | `1` | Ativa `TCP_NODELAY` quando usando TCP |
| `SO_REUSEPORT_ENABLED` | `1` | Ativa `SO_REUSEPORT` se disponível |

### `io_uring`

| Variável | Default | Descrição |
|---|---:|---|
| `IOURING_QD` | `4096` | Queue depth do ring |
| `ACCEPT_SQES` | `256` | Quantidade inicial de accepts pendurados |
| `BACKLOG` | `4096` | Backlog do socket |
| `IOURING_SQPOLL` | `0` | Tenta usar SQPOLL |
| `IOURING_SQPOLL_CPU` | `-1` | CPU para SQPOLL quando habilitado |

### Normalização das features

| Variável | Default | Descrição |
|---|---:|---|
| `AMOUNT_DIVISOR` | `10000.0` | Normalização do valor da transação |
| `INSTALLMENTS_DIVISOR` | `12.0` | Normalização de parcelas |
| `TX24H_DIVISOR` | `20.0` | Normalização da contagem de transações em 24h |
| `KM_DIVISOR` | `1000.0` | Normalização de distâncias em km |
| `MERCHANT_AMOUNT_DIVISOR` | `10000.0` | Normalização da média de valor do merchant |

---

## Respostas HTTP

As respostas são preparadas em `http_responses_init()`.

Para o endpoint de score, existem 6 respostas possíveis pré-geradas:

```text
resp_score[0] -> {"approved":true,"fraud_score":0.0000}
resp_score[1] -> {"approved":true,"fraud_score":0.2000}
resp_score[2] -> {"approved":true,"fraud_score":0.4000}
resp_score[3] -> {"approved":false,"fraud_score":0.6000}
resp_score[4] -> {"approved":false,"fraud_score":0.8000}
resp_score[5] -> {"approved":false,"fraud_score":1.0000}
```

Isso evita `snprintf()` no caminho quente da requisição.

Códigos possíveis:

| Situação | Status |
|---|---|
| `GET /ready` | `200 OK` |
| `POST /fraud-score` válido | `200 OK` |
| JSON inválido ou campos ausentes | `400 Bad Request` |
| Payload maior que `REQ_BUF_SIZE` | `413 Payload Too Large` |
| Rota desconhecida | `404 Not Found` |
| Erro interno inesperado na votação | `500 Internal Server Error` |

---

## Estrutura dos arquivos

```text
src/
├── build_index.c        # gerador do index.bin IVF6
├── common.h             # includes, constantes, helpers e defines globais
├── config.c             # leitura das variáveis de ambiente
├── config.h
├── dataset.c            # carregamento do índice IVF6 em memória
├── dataset.h
├── http_responses.c     # respostas HTTP pré-montadas
├── http_responses.h
├── iouring_server.c     # servidor HTTP com io_uring + TCP/UDS
├── iouring_server.h
├── ivf_search.c         # busca IVF6, bbox repair, top-5, scalar/AVX2
├── ivf_search.h
├── main.c               # bootstrap, carga do índice, fork e server loop
├── mcc_risk.c           # tabela de risco por MCC
├── mcc_risk.h
├── vectorizer.c         # parser JSON mínimo e vetor de 14 dimensões
└── vectorizer.h
```

---

## Build

O projeto depende de `liburing`, `zlib` e de uma toolchain C moderna.

Em sistemas Debian/Ubuntu, as dependências principais são:

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build liburing-dev zlib1g-dev
```

Uma build típica com CMake seria:

```bash
cmake -S . -B build -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build build -j"$(nproc)"
```

O `CMakeLists.txt` atual usa flags voltadas para Haswell, compatíveis com o Mac Mini Late 2014 do desafio:

```text
-O3 -march=haswell -mtune=haswell -flto -fomit-frame-pointer -DNDEBUG
```

Isso habilita AVX2 na CPU alvo sem depender de `-march=native` da máquina onde você está compilando.

---

## Execução

### Rodando com Unix Domain Socket

```bash
INDEX_PATH=resources/index.bin \
MCC_RISK_PATH=resources/mcc_risk.json \
UDS_PATH=/tmp/rinha.sock \
WORKERS=1 \
IVF_NPROBE=1 \
CANDIDATES=0 \
./rinha_backend
```

Teste via `curl` usando socket Unix:

```bash
curl --unix-socket /tmp/rinha.sock \
  -X POST http://localhost/fraud-score \
  -H 'Content-Type: application/json' \
  --data @transaction.json
```

Healthcheck:

```bash
curl --unix-socket /tmp/rinha.sock http://localhost/ready -i
```

### Rodando com TCP

```bash
LISTEN_TCP=1 \
HOST=0.0.0.0 \
PORT=9999 \
INDEX_PATH=resources/index.bin \
./rinha_backend
```

Teste:

```bash
curl -X POST http://localhost:9999/fraud-score \
  -H 'Content-Type: application/json' \
  --data @transaction.json
```

---

## Exemplo de configuração HAProxy com Unix Domain Socket

Exemplo conceitual para duas instâncias:

```haproxy
frontend rinha_front
    bind *:9999
    mode http
    default_backend rinha_back

backend rinha_back
    mode http
    balance roundrobin
    option http-keep-alive
    server api1 /sockets/api1.sock check inter 1s
    server api2 /sockets/api2.sock check inter 1s
```

Cada instância pode ser iniciada com um socket diferente:

```bash
UDS_PATH=/sockets/api1.sock ./rinha_backend
UDS_PATH=/sockets/api2.sock ./rinha_backend
```

---

## Docker Compose

O `docker-compose.yml` atual sobe:

```text
haproxy -> api1
        -> api2
```

Recursos configurados:

```text
api1:    0.40 CPU / 150 MB
api2:    0.40 CPU / 150 MB
haproxy: 0.20 CPU / 50 MB
```

Total:

```text
1.00 CPU / 350 MB
```

As APIs usam Unix Domain Socket compartilhado pelo volume `rinha-sockets`:

```text
/sockets/api1.sock
/sockets/api2.sock
```

---

## Por que essa abordagem é rápida

```text
Baixa latência vem da soma de várias decisões pequenas:

1. C puro
   ↓
2. servidor sem framework
   ↓
3. io_uring para accept/read/write
   ↓
4. keep-alive
   ↓
5. Unix Domain Socket por padrão
   ↓
6. índice inteiro em memória
   ↓
7. IVF reduz quantidade de vetores escaneados inicialmente
   ↓
8. bounding-box repair evita varredura completa na maioria dos casos
   ↓
9. int16 reduz memória e custo de distância
   ↓
10. distância inteira em uint64 evita divergência de float
   ↓
11. layout SoA melhora acesso sequencial
   ↓
12. AVX2 processa vários vetores por vez
   ↓
13. top-5 fixo evita ordenação grande
   ↓
14. respostas HTTP pré-montadas
```

---

## O caminho quente da aplicação

O caminho quente é o que acontece em toda requisição válida de score:

```text
recv
 │
 ▼
parse HTTP mínimo
 │
 ▼
parse JSON mínimo por busca de chaves
 │
 ▼
monta float[14]
 │
 ▼
quantiza query para int16 scale=10000
 │
 ▼
calcula distância para IVF_CLUSTERS centroids
 │
 ▼
seleciona IVF_NPROBE clusters
 │
 ▼
escaneia candidatos dos clusters iniciais
 │
 ▼
repara com bounding boxes se CANDIDATES=0
 │
 ▼
atualiza top-5 por distância e original_id
 │
 ▼
conta labels de fraude
 │
 ▼
escreve resposta já pronta
```

O código evita bibliotecas JSON genéricas e evita alocações no caminho principal da requisição. O parser procura diretamente pelas chaves necessárias e extrai números, strings e booleanos.

---

## Observações importantes

- O servidor espera que o `Content-Length` esteja presente no `POST /fraud-score`.
- O tamanho máximo de requisição é limitado por `REQ_BUF_SIZE`, atualmente `32768` bytes.
- O número máximo de conexões simultâneas no array interno é `MAX_CONNS`, atualmente `4096`.
- O índice precisa ter exatamente `DIM = 14`, `IVF_CLUSTERS = 256`, `FIX_SCALE = 10000.0f` e magic `IVF6`.
- `IVF_K` em `build_index.c` e `IVF_CLUSTERS` em `common.h` precisam ser iguais.
- Se o arquivo `INDEX_PATH` não existir ou for incompatível, a aplicação encerra no startup.
- Se `IOURING_SQPOLL=1` falhar, o código tenta inicializar novamente sem SQPOLL.
- `GET /ready` não valida dependências externas, porque o desenho da aplicação carrega o índice no startup e depois roda em memória.
- `CANDIDATES=0` é o modo normal para usar o repair por bounding-box.

---

## Checklist rápido

Antes de subir a aplicação, confirme:

```text
[ ] resources/index.bin existe
[ ] index.bin foi gerado no formato IVF6
[ ] index.bin foi gerado com FIX_SCALE=10000
[ ] IVF_K do build_index.c bate com IVF_CLUSTERS do common.h
[ ] MCC_RISK_PATH aponta para um JSON válido ou você aceita a tabela default
[ ] UDS_PATH é diferente para cada instância quando há mais de uma API
[ ] HAProxy aponta para os sockets corretos
[ ] permissões do socket permitem acesso pelo HAProxy
[ ] IVF_NPROBE está ajustado para o trade-off latência/qualidade desejado
[ ] CANDIDATES=0 se você quer manter o repair por bounding-box ativo
[ ] binário foi compilado para a CPU correta, especialmente se usar AVX2
```

---

## TL;DR

Este backend é uma implementação enxuta e orientada a performance para detecção de fraude:

```text
JSON de transação
  -> vetor 14D
  -> quantização int16 scale=10000
  -> índice IVF6 em memória
  -> busca em clusters próximos
  -> repair por bounding-box
  -> top-5 determinístico por distância/original_id
  -> votos de fraude
  -> score de 0.0 a 1.0
  -> approved true/false
```

A regra final é simples:

```text
fraud_score = quantidade_de_fraudes_no_top5 / 5
approved = quantidade_de_fraudes_no_top5 < 3
```

Mas o caminho até essa decisão foi desenhado para ser rápido e estável: `io_uring`, Unix Domain Socket, índice vetorial IVF6, quantização `int16` alinhada ao dataset, distância inteira em `uint64`, layout SoA, AVX2 quando disponível, desempate por `original_id`, repair por bounding-box e respostas HTTP pré-montadas.