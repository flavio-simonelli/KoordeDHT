## Deploy locale con Jaeger (tracing analysis)

Questo deployment avvia in locale una rete KoordeDHT minimale utilizzando **Docker Compose**.  
Include:
- **Jaeger** per il tracing distribuito tramite OpenTelemetry.
- **Bootstrap node** come punto di ingresso iniziale della rete.
- **Più nodi Koorde** che formano l’anello DHT.
- **Client interattivo** per eseguire operazioni di `put`, `get`, `delete`, `lookup` e debug.

La configurazione della rete e dei nodi può essere personalizzata modificando il file `common_node.yml` in questa cartella.

### Avvio dell’ambiente

Per avviare l'ambiente con un numero specifico di nodi, esegui:

```bash
docker-compose up --build --scale node=<NUMERO_NODI>
```
Sostituisci `<NUMERO_NODI>` con il numero desiderato di nodi Koorde (ad esempio, `3`).
Il bootstrap node e il client interattivo verranno avviati automaticamente.

### Accesso a Jaeger
Una volta che i container sono in esecuzione, è possibile accedere all'interfaccia di Jaeger tramite il tuo browser web all'indirizzo:

http://localhost:16686

### Utilizzo del client interattivo
Il client interattivo consente di eseguire operazioni sulla rete KoordeDHT. Per accedere al client, esegui:
```bash
docker-compose run --rm client
```
Una volta all'interno del client, puoi utilizzare i seguenti comandi:
- `put <key> <value>`: Inserisce una coppia chiave-valore nella DHT.
- `get <key>`: Recupera il valore associato a una chiave.
- `delete <key>`: Rimuove la coppia chiave-valore dalla DHT.
- `lookup <key>`: Trova il nodo responsabile per una chiave specifica.
- `getrt`: Visualizza la tabella di routing del nodo client.
- `getstore`: Visualizza il contenuto della memoria del nodo client.
- `help`: Mostra l'elenco dei comandi disponibili.
- `exit` o `quit`: Esce dal client interattivo.

Ogni comando eseguito tramite il client verrà tracciato e si potranno visualizzare i dettagli delle operazioni nell'interfaccia di Jaeger.

### Arresto dell’ambiente
Per arrestare l'ambiente e rimuovere i container, esegui:
```bash
docker-compose down
```

### Note
- Tutti i nodi condividono la stessa configurazione (`common_node.env`).
- Il nodo bootstrap viene avviato per primo e funge da punto di contatto iniziale.
- Gli altri nodi si connettono al bootstrap tramite la variabile d’ambiente `BOOTSTRAP_PEERS=bootstrap:4000`.
- I trace OpenTelemetry vengono inviati automaticamente a Jaeger sulla porta `4317`.