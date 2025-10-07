## Deploy Dimostrativo Multi-EC2 con Route53

Questa configurazione rappresenta la modalità **dimostrativa** del sistema KoordeDHT, progettata per riprodurre un contesto realistico di produzione.  
In questo setup, **ogni istanza EC2** ospita **più container Koorde** che si registrano automaticamente su **AWS Route53**, permettendo al client di interagire con la DHT conoscendo solo l’indirizzo DNS di un nodo.


##  Architettura generale

- Ogni **istanza EC2** esegue:
    - Un **cluster locale** di container Koorde (N nodi)
    - Docker e Docker Compose installati automaticamente
    - Registrazione DNS automatica su Route53 tramite `ROUTE53_ZONE_ID` e `ROUTE53_SUFFIX`
- I container pubblicano la porta `BASEPORT+N` (una per nodo)
- Tutte le istanze EC2 appartengono alla stessa **VPC** e **Hosted Zone** Route53, creando una rete distribuita accessibile da un client esterno

## Prerequisiti

- Un **bucket S3** per caricare gli script e i file di configurazione.
- Installare [AWS CLI](https://aws.amazon.com/cli/) e configurarlo con le credenziali `~/.aws/credentials`.
- Aver creato una **VPC** con almeno una subnet che è possibile usare per le istanze EC2.
- Aver creato una **Hosted Zone** privata in Route53 associata alla VPC.
- Avere una **routing table** che consenta la comunicazione tra le istanze EC2.
- Avere un **Internet Gateway** associato alla VPC per l'accesso a internet (per scaricare Docker, ecc.).
- Aver creato un **ruolo IAM** con permessi di gestione di Route53 e associato alle istanze EC2.
- Avere una **key pair** per accedere alle istanze EC2 via SSH.
- Aver caricato tutto il contenuto della cartella `deploy/demonstration/scripts` in una cartella di un bucket S3.

### Generazione del file docker-compose
Lo script `gen_compose.sh` genera automaticamente un file `docker-compose.generated.yml` a partire dal template, sostituendo i parametri di simulazione e replicando il campo node un numero di volte pari al unemro di nodi che si vuole istanziare per ogni EC2.
Infatti in questo deployment ogni istanza EC2 esegue più nodi Koorde e ognuno di questi deve essere associata una porta differente per poter essere contattato dall'esterno (dagli altri nodi e dal client).
Esempio di esecuzione:
```bash
./gen_compose.sh \
  --nodes 5 \
  --base-port 4000 \
  --mode private \
  --zone-id ZxxxxxxxxxxxxxxC \
  --suffix dht.local \
  --region eu-west-1
```
Il file risultante sarà salvato come `docker-compose.generated.yml` e verrà utilizzato da `init.sh`.

### Script di orchestrazione
Lo script `init.sh` coordina il deploy all'interno di una singola istanza EC2:
- Scarica e installa Docker e Docker Compose
- Crea un file `docker-compose.generated.yml` personalizzato per l'istanza
- Avvia i container Koorde

Prima dell’esecuzione, devono essere impostate le variabili d’ambiente:
```bash
export NODES=5
export BASE_PORT=4000
export MODE=private
export ROUTE53_ZONE_ID=Zxxxxxxxxxxxx12C
export ROUTE53_SUFFIX=koorde-dht.local
export ROUTE53_REGION=eu-east-1
export S3_BUCKET=koorde-bucket
export S3_PREFIX=demonstration
```
Tutti i log sono salvati in `/var/log/init.log`.

### Template CloudFormation
Il file `koorde.yml` è un template CloudFormation che crea un'istanza EC2 con Docker e Docker Compose installati e scarica lo script `init.sh` da bucket S3 e lo avvia automaticamente.

### Deploy su AWS
Per eseguire il deploy della rete KoordeDHT, utilizza lo script `deply_koorde.sh`:
```bash
./deploy-koorde.sh \
  --instances 5 \
  --nodes 5 \
  --base-port 4000 \
  --mode private \
  --zone-id ZxxxxxxxxxxxxxxxxR \
  --region us-east-1 \
  --suffix koorde-dht.local \
  --s3-bucket koorde-bucket \
  --s3-prefix demonstration \
  --key-name Amazon-Key \
  --instance-type t2.micro \
  --vpc-id vpc-0xxxxxxxxxxxxxxxf \
  --subnet-id subnet-0xxxxxxxxxxxxx0
```

### Accesso alla rete KoordeDHT
Dopo che le istanze EC2 sono state avviate, è possibile utilizzare il client interattivo per eseguire operazioni sulla rete KoordeDHT.  
Per accedere al client, esegui:
```bash
docker run -it --rm koordectl:latest client --addr <NODO_BOOTSTRAP>:<PORTA>
```
Sostituire `<NODO_BOOTSTRAP>` con l'indirizzo pubblico di una delle istanze e `<PORTA>` con la porta associata a quel nodo (ad esempio, `4000`).
Una volta all'interno del client, puoi utilizzare i seguenti comandi:
- `put <key> <value>`: Inserisce una coppia chiave-valore nella DHT.
- `get <key>`: Recupera il valore associato a una chiave.
- `delete <key>`: Rimuove la coppia chiave-valore dalla DHT.
- `lookup <key>`: Trova il nodo responsabile per una chiave specifica.
- `getrt`: Visualizza la tabella di routing del nodo client.
- `getstore`: Visualizza il contenuto della memoria del nodo client.
- `help`: Mostra l'elenco dei comandi disponibili.
- `exit` o `quit`: Esce dal client interattivo.

### Arresto della rete
Per arrestare la rete KoordeDHT, è possibile eliminare lo stack CloudFormation creato con lo script `destroy_koorde.sh`:
```bash
./destroy_koorde.sh
```
Questo comando eliminerà tutte le istanze EC2 e le risorse associate create durante il deploy.

### Note
Nella caretella `deploy/demonstration/results` sono presenti esempi di output del client derivanti dall'esecuzione della dimostrazione.