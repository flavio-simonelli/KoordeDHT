# KoordeDHT – Implementazione Distribuita del De Bruijn Overlay

KoordeDHT è un’implementazione moderna e containerizzata del protocollo **Koorde**, una Distributed Hash Table (DHT) basata su **de Bruijn graphs**, derivata dal modello **Chord**.  
Il progetto è sviluppato in **Go** e integra strumenti DevOps per **deploy automatizzato**, **simulazione di rete**, e **monitoraggio distribuito**.

## Riferimenti teorici

L’implementazione si basa sul paper originale:

> **Kaashoek MF, Karger DR.** *Koorde: A simple degree-optimal distributed hash table.*  
> MIT Laboratory for Computer Science (2003)

Koorde raggiunge le seguenti proprietà teoriche:
- Grado costante con **O(log n)** hop per lookup
- Possibilità di aumentare il grado per ottenere **O(log n / log log n)** hop
- Basso overhead di manutenzione e routing deterministico
- Supporto alla fault tolerance tramite liste di successori

## Architettura del progetto

KoordeDHT è organizzato in microservizi containerizzati:

| Servizio | Descrizione |
|-----------|-------------|
| **koorde-node** | Nodo DHT principale, con routing de Bruijn e registrazione opzionale su Route53 |
| **koorde-client** | Client interattivo gRPC per eseguire operazioni (`put`, `get`, `delete`, `lookup`, `getrt`, `getstore`) |
| **koorde-tester** | Client automatico per test su larga scala, generazione CSV e misure di latenza |

Vengono in oltre utilizzati i seguenti servizi di supporto opensource containerizzati:

| Servizio | Descrizione |
|-----------|-------------|
| **jaeger** | Servizio di tracing distribuito (OpenTelemetry) |
| **pumba** | Strumento di network chaos per simulare latenza, jitter e perdita di pacchetti |

Viene utilizzato AWS per il deploy in cloud, con i seguenti servizi:

| Servizio | Descrizione |
|-----------|-------------|
| **EC2** | Istanze virtuali per eseguire i nodi Koorde |
| **Route53** | Servizio DNS per la registrazione automatica dei nodi |
| **S3** | Storage per gli script di deploy e configurazione |
| **CloudFormation** | Template per il deploy automatizzato dell’infrastruttura |
| **VPC** | Rete privata per la comunicazione tra le istanze EC2 |


## Funzionalità principali

-  **Implementazione completa del routing Koorde** (base-k, logica di “imaginary hops” e successor correction)
-  **Simulazione di churn** dinamico con controllore dedicato
-  **Deploy multi-istanza AWS con registrazione DNS automatica su Route53**
-  **Tracciamento distribuito** con Jaeger e OpenTelemetry (gRPC + custom metadata)
-  **Test automatizzati** con raccolta di metriche CSV e visualizzazione in Grafana/Tempo
-  **Gestione centralizzata tramite script** Bash per installazione, setup, teardown e logging


## Modalità di Deploy

KoordeDHT supporta 3 diverse modalità di deploy, la loro documentazione è disponibile nelle rispettive cartelle:
- [Deploy locale con Jaeger (tracing analysis)](deploy/tracing/README.md)
- [Deploy di Test (Simulazione automatizzata con churn e ritardi di rete)](deploy/test/README.md)
- [Deploy dimostrativo su AWS (multi-istanza con Route53)](deploy/demonstration/README.md)

