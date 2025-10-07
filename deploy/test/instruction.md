## Deploy di Test (Simulazione automatizzata con churn e ritardi di rete)

Questa configurazione consente di testare il comportamento della DHT in condizioni di **rete realistiche** e con **churn dinamico** (entrata e uscita casuale dei nodi).  
Il sistema è completamente automatizzato e può essere eseguito sia in locale che su **istanze EC2** AWS.

Include:
- Script di generazione automatica del file `docker-compose` con parametri di simulazione.
- Controller di **churn** (`churn.sh`) per interrompere e riavviare nodi in modo casuale.
- Container **Pumba** per la simulazione del ritardo e del jitter di rete.
- **Tester automatico** che invia richieste di lookup e raccoglie metriche CSV.
- Script di orchestrazione (`init.sh`) che coordina l’intera simulazione.
- Template **CloudFormation** e launcher AWS per il deploy in cloud.

---

## Prerequisiti
- Su AWS bisogna avere un **bucket S3** per caricare gli script e i file di configurazione.
- Installare [AWS CLI](https://aws.amazon.com/cli/) e configurarlo con le credenziali.
- Aver creato una VPC con almeno una subnet che è possibile usare per la istanza EC2.

### Generazione del file docker-compose
Lo script `gen_compose.sh` genera automaticamente un file `docker-compose.generated.yml` a partire dal template, sostituendo i parametri di simulazione.

Esempio di esecuzione:
```bash
./gen_compose.sh \
  --sim-duration 60s \
  --query-rate 0.5 \
  --query-parallelism-min 1 \
  --query-parallelism-max 5 \
  --query-timeout 10s \
  --docker-suffix test
```
Il file risultante sarà salvato come `docker-compose.generated.yml` e verrà utilizzato da `init.sh`.

### Simulazione del churn
Lo script `churn.sh` gestisce il churn dei nodi, fermando e riavviando nodi in modo casuale durante la simulazione utilizzando docker ps su container con un prefisso specificato.
Esempio di esecuzione:
```bash
./churn.sh apply -p koorde-node- -i 20 -m 3 -j 0.4 -l 0.3
```
Significato dei parametri:
- `-p` prefisso dei container (es. koorde-node-)
- `-i` intervallo in secondi tra gli eventi
- `-m` minimo numero di nodi attivi da mantenere
- `-j` probabilità di join
- `-l` probabilità di leave

Se la probabilità di join e leave è 0.4 e 0.3, rispettivamente, c'è il 40% di probabilità che un nodo venga riavviato e il 30% che un nodo venga fermato ad ogni intervallo e rimane un 30% di probabilità che lo script non faccia nulla in quell'intervallo.

Lo script continuerà a eseguire il churn fino a quando:
- non viene interrotto manualmente (ad esempio con Ctrl+C)
- venga killato il processo (approccio utilizzato in `init.sh`)
- venga usato il comando `churn.sh clear` per terminare il processo di churn.

### Simualzione del ritardo di rete
Il container **Pumba** viene utilizzato per introdurre ritardi di rete, jitter e packet loss tra i nodi.  
Il container viene avviato durante l'esecuzione di `init.sh` e applica le regole di rete ai container dei nodi Koorde.

### CloudFormation e deploy su AWS
Il file `test_koorde.yml` è un template CloudFormation che crea un'istanza EC2 con Docker e Docker Compose installati e scarica lo script `init.sh` da bucket S3 e lo avvia automaticamente.

### Script di orchestrazione
Lo script `init.sh` coordina l'intera simulazione:
- Scarica tutti gli script e i file necessari da S3 (se eseguito su AWS).
- Installa Docker e Docker Compose utilizzando lo script `install_docker.sh`.
- Genera il file `docker-compose.generated.yml` con i parametri specificati.
- Avvia i container con Docker Compose.
- Avvia il churn dei nodi.
- Avvia il il container Pumba per la simulazione della rete.
- Aspetta il completamento della simulazione.
- Salva i log e i risultati in due file che carica all'interno dello stesso bucket S3 in una cartella denominata timestamp.

Esempio di esecuzione:
```bash
./init.sh \
  --bucket koorde-bucket \
  --prefix test \
  --sim-duration 5m \
  --query-rate 0.5 \
  --parallel-min 1 \
  --parallel-max 5 \
  --delay 200ms \
  --jitter 50ms \
  --loss 0.1% \
  --churn-interval 20 \
  --churn-min-active 3 \
  --churn-pjoin 0.4 \
  --churn-pleave 0.3 \
  --max-nodes 10
```

# Avvio della simulazione

Per avviare la simulazione è necessario come prerequisito:
- Aver caricato precedentemente gli script e i file di configurazione che si trovano nella sottocartella `deploy/test/scripts` all'interno di un bucket S3 tutti all'interno di una cartella.
- Aver creato una VPC con almeno una subnet che è possibile usare per la istanza EC2.
- Inserito le credenziali AWS nel file `~/.aws/credentials`.

Successivamente, è possibile eseguire lo script `deploy_test.sh` per avviare la simulazione su AWS:
```bash
./deploy_test.sh \
  --keypair Amazon-Key \
  --s3-bucket koorde-bucket \
  --s3-prefix test \
  --vpc-id vpc-xxxxxxxxxxxxxebef \
  --subnet-id subnet-xxxxxxxxxxxd4090 \
  --instance-type t2.small \
  --sim-duration 5m \
  --query-rate 0.8 \
  --parallel-min 1 \
  --parallel-max 5 \
  --delay 100ms \
  --jitter 50ms \
  --loss 0.1% \
  --churn-interval 15 \
  --churn-min-active 5 \
  --churn-pjoin 0.5 \
  --churn-pleave 0.5 \
  --max-nodes 30
```
Questo script lancierà il template di CloudFormation `test_koorde.yml` con i parametri specificati e avvierà la simulazione sull'istanza EC2 creata.

è possibibile monitorare lo stato della istanza EC2 dalla console AWS e collegarsi via SSH per visualizzare i log in tempo reale.

- Tutti i log sono salvati in `/var/log/test/`.
- I risultati CSV generati dal tester sono salvati in `./results/output.csv`.
- In caso di esecuzione su EC2, i file vengono automaticamente caricati su `s3://<bucket>/<prefix>/results/`.

### Arresto della simulazione
Per arrestare la simulazione, è possibile terminare l'istanza EC2 dalla console AWS o eseguire il comando:
```bash
./destroy_test.sh
```

Questo script cancellerà lo stack CloudFormation creato e terminerà l'istanza EC2 associata.

#### Note
- Sono disponibili alcuni esempi di output CSV e log nella cartella `deploy/test/results`, ta cui i risultati utilizzati per la docuementazione.



