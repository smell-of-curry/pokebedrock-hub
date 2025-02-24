
# PokeBedrock Moderation System

This document describes the PokeBedrock Moderation System API, including configuration details, data structures, endpoints, and TypeScript typings for integration.

---

## 1. Base Configuration

- **Base URL:**  
  `https://pokebedrock.com`

- **Required Headers (for every request):**  
  - **authorization:** API token (supplied via environment variables).  
  - **Content-Type:** `application/json`

---

## 2. Identification Requirements

Every request to the moderation endpoints must include **at least one** of the following identification fields to locate a player's record:

- **xuid:** Unique player identifier.
- **name:** Player's gamertag or known name.
- **discord_id:** Player's Discord ID.
- **ip:** A valid IP address.  
  > **Note:** If an IP is provided, it will be stored internally as an array (`ips`).

---

## 3. Data Structures

### 3.1 Infliction Object

An **Infliction** represents a punishment issued to a player. It must adhere to the following schema:

- **type (string):**  
  Must be one of:
  - `"BANNED"`
  - `"MUTED"`
  - `"FROZEN"` (prevents movement)
  - `"WARNED"` (a warning only)
  - `"KICKED"` (player removed from the server)

- **date_inflicted (number):**  
  Unix timestamp (in milliseconds) indicating when the infliction was applied. Must be a non-negative integer.

- **expiry_date (number | null):**  
  Unix timestamp (in milliseconds) when the infliction expires, or `null` for permanent actions. If provided, it must be greater than `date_inflicted`.

- **reason (string):**  
  Description of why the infliction was issued.

- **prosecutor (string):**  
  Identifier for who issued the infliction (e.g., `"System"` or an administrator’s name).

### 3.2 Moderation Record

A player's moderation record contains the following fields:

- **Player Identification:**  
  - `xuid`
  - `name`
  - `discord_id`
- **ips:**  
  An array of known IP addresses.
- **current_inflictions:**  
  Array of active **Infliction** objects.
- **past_inflictions:**  
  Array of historical **Infliction** objects.
- **date_created / date_updated:**  
  Timestamps (managed by the backend) for record creation and last update.

---

## 4. API Endpoints

### 4.1 Add Infliction

- **Endpoint:**  
  `POST https://pokebedrock.com/api/moderation/addInfliction`

- **Purpose:**  
  Create or update a player's moderation record by adding a new infliction.

- **Request Body (JSON):**
  ```json
  {
    // Identification: include at least one field (xuid, name, discord_id, ip)
    "xuid": "player_xuid",
    "name": "player_name",
    "discord_id": "player_discord_id",
    "ip": "player_ip_address",

    "inflictionStatus": "current", // or "past"
    "infliction": {
      "type": "BANNED",
      "date_inflicted": 1610000000000,
      "expiry_date": 1610003600000, // or null for permanent
      "reason": "Violation of rules",
      "prosecutor": "AdminName"
    }
  }
  ```

- **Behavior:**
  - If no record exists, a new moderation record is created.
  - If a record exists, the new infliction is appended to either `current_inflictions` or `past_inflictions` (based on `inflictionStatus`).
  - If multiple records are found, they are merged before the new infliction is added.

- **Response:**  
  HTTP `204 No Content` on success.

---

### 4.2 Remove Infliction

- **Endpoint:**  
  `DELETE https://pokebedrock.com/api/moderation/removeInfliction`

- **Purpose:**  
  Remove an active infliction by moving it to the historical records.

- **Request Body (JSON):**
  ```json
  {
    // Identification: include at least one field (xuid, name, discord_id, ip)
    "xuid": "player_xuid",
    "name": "player_name",
    "discord_id": "player_discord_id",
    "ip": "player_ip_address",

    "infliction": { // Infliction to be removed.
      "type": "MUTED",
      "date_inflicted": 1610000000000,
      "expiry_date": null, // This field is ignored during comparison.
      "reason": "Spamming",
      "prosecutor": "System"
    }
  }
  ```

- **Behavior:**
  - The system finds the player’s moderation record using the provided identification.
  - The matching infliction is removed from `current_inflictions` and added to `past_inflictions`.
  - If multiple records match, they are merged before the infliction is processed.

- **Response:**  
  HTTP `204 No Content` on success.

---

### 4.3 Get Inflictions

- **Endpoint:**  
  `GET https://pokebedrock.com/api/moderation/getInflictions`

- **Purpose:**  
  Retrieve a player's complete moderation record, including both active and past inflictions.

- **Request Body (JSON):**
  ```json
  {
    // Identification: include at least one field (xuid, name, discord_id, ip)
    "xuid": "player_xuid",
    "name": "player_name",
    "discord_id": "player_discord_id",
    "ip": "player_ip_address"
  }
  ```

- **Response:**  
  HTTP `200 OK` with a JSON object:
  ```json
  {
    "current_inflictions": [ /* Array of Infliction objects */ ],
    "past_inflictions": [ /* Array of Infliction objects */ ]
  }
  ```

---

## 5. TypeScript Types

The following TypeScript definitions are used in the PokeBedrock addon implementation:

```ts
// Enumeration for valid infliction types.
export enum InflictionTypes {
  BANNED = "BANNED",
  MUTED = "MUTED",
  FROZEN = "FROZEN", // Prevents player movement.
  WARNED = "WARNED", // Warning only.
  KICKED = "KICKED", // Player removed from the server.
}

// Infliction object structure.
export interface Infliction {
  type: InflictionTypes;
  date_inflicted: number;
  /**
   * Unix timestamp (in ms) when the infliction expires.
   * Use `null` for permanent actions.
   */
  expiry_date: number | null;
  reason: string;
  prosecutor: string | null;
}

// Complete moderation record for a player.
export interface PlayerModerationStatus {
  xuid: string; // Unique player identifier.
  name: string; // Gamertag or known name.
  ips: string[]; // Known IP addresses.
  discord_id: string | null; // Discord ID (if available).
  current_inflictions: Infliction[];
  past_inflictions: Infliction[];
  date_created: number;
  date_updated: number;
}

// Identification fields used when querying or updating records.
export interface GlobalModerationPlayerInformation {
  xuid?: string;
  name?: string;
  ips?: string[];
  discord_id?: string;
}
```