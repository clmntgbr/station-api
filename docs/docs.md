# Carbu API — Documentation d'architecture

Oct 12, 2026 · @Clement

## Contexte et objectifs

Carbu API expose les prix des carburants autour d'une position GPS à partir des données ouvertes roulez-eco, puis sera étendue aux bornes de recharge électrique.

- **Phase 1** : ingestion régulière du flux instantané, recherche géographique par distance et par carburant.
- **Phase 2** : liaison de chaque station à Google Places pour enrichir sa fiche (nom, enseigne, horaires, photos).
- **Ensuite** : bornes de recharge électrique dans le même modèle de données.
- **Stack** : Go, event-driven + CQRS (même structure que FlowForge et leakcut), RabbitMQ, PostgreSQL/PostGIS, auth Clerk.

## Source de données roulez-eco

Le flux instantané `https://donnees.roulez-eco.fr/opendata/instantane` est un ZIP contenant un XML encodé en ISO-8859-1, avec une balise `pdv` par station.

| Élément | Contenu | Remarque |
| --- | --- | --- |
| `id` | Identifiant de la station (ex. 89100001) | Stocké dans `station_id`. La clé primaire `id` est un UUID v7 |
| `latitude`, `longitude` | Degrés × 100 000 (4818300 → 48,183) | Parfois décimaux (4675351.71497) |
| `cp`, `adresse`, `ville` | Adresse postale |  |
| `horaires` | Attribut `automate-24-24` + 7 jours | Les automates portent `ferme="1"` avec un créneau 01.00–01.00 |
| `services` | Liste de libellés | Peut être vide |
| `prix` | `nom`, `id`, `maj`, `valeur` en €/L | Gazole 1, SP95 2, E85 3, GPLc 4, E10 5, SP98 6 |

Pièges relevés sur l'échantillon de 5 stations :

- **Encodage** : décoder explicitement l'ISO-8859-1, sinon « Pézenas » devient illisible.
- **Coordonnées** : diviser par 100 000 quand la valeur dépasse 180, rejeter les stations sans position valide.
- **Stations sans prix** : la station 58240003 n'en a aucun, elle est stockée sans prix.
- **Fraîcheur inégale** : à Sens (89100001), le Gazole date du 11/10 mais le SP98 du 28/09. La date `maj` se conserve par carburant et s'affiche au front.
- **Volume** : lire le fichier en streaming (décodeur XML par jetons), sans le charger entier en mémoire.
- **À confirmer** : fréquence exacte de mise à jour du flux et conditions de la licence (attribution).

## Events et contrats de messages

Tous les échanges passent par RabbitMQ sous forme d'events versionnés, dans une enveloppe commune `{name, occurred_at, payload}`.

| Event | Producteur | Consommateur | Contenu |
| --- | --- | --- | --- |
| `station.upserted.v1` | scheduler (`fuel_feed`), CLI `feed pull` | worker | `id` (uuid v7), `stationId`, station complète + hash |
| `station.price_changed.v1` | worker (outbox) | futurs consommateurs (alertes) | `id` (uuid v7), `stationId` (identifiant roulez-eco), carburant, ancien et nouveau prix |
| `station.enrichment_requested.v1` | API (recherche) | enricher (phase 2) | `id` (uuid v7) |
| `station.enriched.v1` | enricher (phase 2) | worker | `google_place_id` + infos Places |

Le binaire `cmd/scheduler` tire le flux roulez-eco toutes les 5 minutes (UTC, bornes `:00` / `:05` / …, configurable via `SCHEDULER_FEED_INTERVAL`). D'autres jobs s'ajoutent en lançant un `go runJob(...)` dans `main`. Un `make feed` / `cli feed pull` reste possible à la main.

Règles de fonctionnement :

- Exchange `topic` unique, une queue par consommateur, dead-letter queue et retries avec délai.
- Le pull ne publie `station.upserted.v1` que si le hash de la station diffère de celui en base : en régime normal, seules les stations modifiées circulent.
- Le worker est idempotent : rejouer un event ne change pas l'état final.
- `station.price_changed.v1` passe par un outbox transactionnel (même transaction que l'écriture du prix), comme dans FlowForge.

## CQRS et modèle de données

Seul le worker écrit en base ; l'API ne fait que lire, sur des tables déjà prêtes pour la requête géographique.

| Table | Rôle | Colonnes clés |
| --- | --- | --- |
| `stations` | Agrégat + projection de lecture | `id` (uuid v7), `station_id` (identifiant roulez-eco), `energy_type`, `location` (geography Point 4326), `services`, `hours` (jsonb), `info_hash`, `price_hash`, `google_place_id` |
| `current_prices` | Prix courants | PK (`station_id`, `fuel`) — `station_id` référence `stations.id`, `price` numeric(6,3), `source_updated_at`, `created_at`, `updated_at` |
| `prices_history` | Historique des prix | `station_id` (uuid, référence `stations.id`), `fuel`, `price`, `source_updated_at`, `observed_at` |

Index : GIST sur `stations.location`, et `(fuel, price)` sur les prix courants pour le tri par prix.

Traitement d'un `station.upserted.v1` par le worker, en une transaction :

1. Upsert de la station (position, adresse, services, horaires, hash).
2. Pour chaque prix, comparaison avec le prix courant.
3. Si le prix diffère : insertion dans l'historique, mise à jour du prix courant, écriture de l'event `station.price_changed.v1` dans l'outbox.

Choix pragmatique : `stations` et `current_prices` font office de modèle de lecture dans la même base ; une séparation physique ne se justifie que si la charge de lecture l'impose.

La colonne `energy_type` (`fuel` ou `ev`) prépare l'arrivée des bornes électriques sans changer le schéma.

## API HTTP et authentification

Un endpoint unique renvoie les stations autour d'une position, triées par distance ou par prix : `GET /v1/stations`.

| Paramètre | Obligatoire | Description |
| --- | --- | --- |
| `lat`, `lng` | oui | Position de l'utilisateur (degrés décimaux) |
| `radius_km` | non | Rayon de recherche, 10 par défaut, 50 maximum |
| `fuel` | non | Carburant (Gazole, SP95, SP98, E10, E85, GPLc) : filtre les stations qui le vendent |
| `sort` | non | `distance` (défaut) ou `price` (nécessite `fuel`) |
| `limit` | non | 20 par défaut, 100 maximum |

Exemple de réponse (une station) :

```json
{
  "id": "34150003",
  "address": "Route de Pézenas",
  "city": "GIGNAC",
  "lat": 43.651,
  "lng": 3.547,
  "distance_m": 2340,
  "automate_24": true,
  "services": ["Station de gonflage", "Lavage automatique"],
  "prices": [
    { "fuel": "Gazole", "price": 2.334, "updated_at": "2026-10-12T08:30:46Z" }
  ]
}
```

Côté SQL, le filtre utilise `ST_DWithin` sur la colonne `geography` (index GIST) et le tri `ST_Distance`.

Authentification : Clerk. Le front envoie le JWT en `Authorization: Bearer`, un middleware Fiber le vérifie et rejette les requêtes sans token valide. Un drapeau de configuration permet de la désactiver en développement.

Côté front : `navigator.geolocation` fournit les coordonnées ; en cas de refus, prévoir une saisie manuelle de ville ou de code postal (service de géocodage à choisir).

## Phase 2 : liaison Google Places

Chaque station reçoit un `google_place_id` qui sert de clé pour récupérer des informations que roulez-eco ne fournit pas (nom commercial, enseigne, photos, horaires détaillés).

- **Matching** : recherche Nearby ou Text Search avec un biais de position (50 à 100 m) et l'adresse de la station, puis score de confiance (distance, similarité d'adresse et de ville). Sous le seuil, la station part en file de revue manuelle.
- **Déclenchement à la demande** : quand une station sans `place_id` apparaît dans une recherche, l'API publie `station.enrichment_requested.v1`. L'enricher traite la file avec un rate limiting. Pas d'enrichissement en masse, pour maîtriser la facture Places.
- **Écriture** : l'enricher publie `station.enriched.v1`, le worker écrit en base, comme pour toute autre donnée.
- **Contraintes Google à vérifier avant d'implémenter** : seul le `place_id` est stockable durablement, le reste (photos, horaires, notes) doit rester en cache de courte durée, et les attributions sont obligatoires.
- **Prix** : la source des prix reste roulez-eco ; Google n'apporte que de l'information descriptive.

## Bornes électriques, décisions et points ouverts

Les bornes électriques s'ajoutent sans refonte : même pipeline d'events, même endpoint avec un paramètre d'énergie, et `energy_type = ev` sur la station.

- **Source probable** : la base consolidée des infrastructures de recharge (IRVE) sur data.gouv.fr, à confirmer.
- **Offre** : au lieu d'un prix par carburant, une borne porte des connecteurs (type, puissance en kW) et un tarif, dans une table dédiée.
- **Ingester** : un second ingester spécialisé, qui publie les mêmes `station.upserted.v1`.

| Sujet | Décision | Statut |
| --- | --- | --- |
| Bus de messages | RabbitMQ | Décidé |
| Authentification | Clerk | Décidé |
| Base de données | PostgreSQL + PostGIS | Proposé |
| Enrichissement Google Places | À la demande, pas en masse | Proposé |
| Lecture des données | Même base que l'écriture (projections) | Proposé |

Points ouverts :

- Fréquence exacte de mise à jour du flux et conditions de la licence.
- GORM (comme FlowForge) ou pgx avec SQL à la main pour les requêtes PostGIS.
- Service de géocodage pour la saisie manuelle d'une ville ou d'un code postal.
- Durée de rétention de l'historique des prix.
- Rate limiting par utilisateur Clerk sur l'API.
