# Dockerfile Reference

Questa sezione descrive i **Dockerfile** principali utilizzati per costruire e distribuire i container del progetto **KoordeDHT**.  
Le immagini compilate sono disponibili pubblicamente su **Docker Hub** nel repository:  
[`flaviosimonelli/koorde`](https://hub.docker.com/r/flaviosimonelli)

---

## 1. `client.Dockerfile`

### Scopo
Crea l’immagine del **client interattivo** della DHT (`koorde-client`), utilizzato per eseguire comandi `put`, `get`, `delete`, `lookup`, e per interrogare routing table e storage di un nodo.

### Struttura

- **Fase builder:**  
  Usa l’immagine base `golang:1.25` per compilare il binario Go:
  ```bash
  CGO_ENABLED=0 GOOS=linux go build -o /koorde-client ./cmd/client
    ```
- **Fase runtime:**
    Usa l’immagine minimale `gcr.io/distroless/base-debian12` per eseguire il binario in un ambiente leggero.
    Il binario viene compilato in `/usr/local/bin/koorde-client`.

### Esecuzione

Per impostazione predefinita mostra l’help:
```bash
docker run --rm flaviosimonelli/koorde-client:latest --help
```

Per eseguire il build manuale:
```bash
docker build -f docker/client.Dockerfile -t flaviosimonelli/koorde-client:latest .
```
---

## 2. `node.Dockerfile`

### Scopo
Crea l’immagine del **nodo della DHT** (`koorde-node`), che esegue un nodo della rete KoordeDHT.

### Struttura
- **Fase builder:**  
  Usa l’immagine base `golang:1.25` per compilare il binario Go:
  ```bash
  CGO_ENABLED=0 GOOS=linux go build -o /koorde-node ./cmd/node
    ```
- **Fase runtime:**
    Usa l’immagine minimale `gcr.io/distroless/base-debian12` per eseguire il binario in un ambiente leggero.
    Il binario viene compilato in `/usr/local/bin/koorde`.

### Esecuzione

Il build include una configurazione yaml all'interno del nodo, che può essere sovrascritta montando un file di configurazione personalizzato tramite variabili d'ambiente.
Le build pubblicate su Docker Hub sono preconfigurate con un config.yaml vuoto, di conseguenza per far funzionare il nodo è necessario fornire un file di configurazione.

Per compilare manualmente l’immagine con un file di configurazione personalizzato:
```bash
docker build -f docker/node.Dockerfile -t flaviosimonelli/koorde-node:latest .
```
avendo cura di modificare il file config.yaml in `/config/node/config.yaml`.

Nella stessa directory è possibile trovare un file di esempio `structure.env` che indica tutte le possibili variabili d'ambiente configurabili.

---

## 3. `node.netem.Dockerfile`

### Scopo
Crea l’immagine del **nodo della DHT con emulazione di rete** (`koorde-node-netem`), che esegue un nodo della rete KoordeDHT con emulazione di rete tramite `tc netem`.
Utile per test di latenza, jitter e perdita pacchetti con strumenti come Pumba.
### Struttura
- **Fase builder:**
come in `node.Dockerfile`
- **Fase runtime:**
    Usa l’immagine `debian:12-slim` per eseguire il binario in un ambiente Debian minimale.
    Installa `iproute2` per poter utilizzare `tc netem`.
    Il binario viene compilato in `/usr/local/bin/koorde`.

### Build manuale

Per compilare manualmente l’immagine con un file di configurazione personalizzato:
```bash
docker build -f docker/node.netem.Dockerfile -t flaviosimonelli/koorde-node-netem:latest .
```
avendo cura di modificare il file config.yaml in `/config/node/config.yaml`.

---

## 4. `tester.Dockerfile`

### Scopo
Crea l’immagine del **tester della DHT** (`koorde-tester`), utilizzato per eseguire test automatici di `lookup` su un nodo della rete KoordeDHT.
### Struttura
- **Fase builder:**
Compila `koorde-tester` da `./cmd/tester`.
- **Fase prep:**
Utilizza `busybox` per creare la directory `/data/results` e gestire permessi.
- **Fase runtime:**
    Usa l’immagine minimale `gcr.io/distroless/base-debian12` per eseguire il binario in un ambiente leggero.

Il container viene eseguito come root (necessario per accedere a `/var/run/docker.sock` durante i test).

### Build manuale
Per compilare manualmente l’immagine:
```bash
docker build -f docker/tester.Dockerfile -t flaviosimonelli/koorde-tester:latest .
```