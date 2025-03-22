# PokeBedrock Server Hub 🎮🌐

The **PokeBedrock Server Hub** is a robust, high-performance central lobby system built with [Dragonfly](https://github.com/df-mc/dragonfly). It serves as the gateway into the expansive PokeBedrock network—seamlessly managing server connections, player queues, and navigation between the main servers.

## Features 🛠️

### 🤖 NPC Slapper System
Interactive NPCs throughout the world let players quickly join various servers across the network.

- **Dynamic Nametags 🔥**
  - **Real-Time Updates:** Display current online player counts.
  - **Status Indicators:** Visual cues for online/offline status and server capacity.
  
- **Smart Interaction 🤝**
  - **Seamless Transfers:** Players can tap on NPCs to initiate server transfers.
  - **Confirmation Dialogs:** Prevent accidental transfers.
  - **Automatic Queueing:** Get placed in line if a server is full.

### ⏳ Priority Queue System
Manage high-traffic periods with a sophisticated priority queue system that ensures smooth and fair player transfers.

- **Visual Queue Display 👀**
  - **Boss Bar:** Shows your current queue position, estimated wait time, and destination.
  - **Real-Time Updates:** Queue positions adjust dynamically based on priority algorithms.
  
- **Priority Rankings 🥇**
  - **Ranked Order:** Players are sorted by rank (ex. Admin > Moderator > Premium > Trainer).
  - **Fairness:** Within the same rank, join time determines order (first come, first served).
  - **Clear Communication:** Queue positions are always clearly indicated.
  
- **Intelligent Transfer 🚀**
  - **Auto-Transfer:** Players are moved automatically as soon as space is available.
  - **Graceful Handling:** Failed transfers trigger automatic re-entry into the queue.
  - **Admin Bypass:** Staff can bypass queues to access near-capacity servers.

### 🧭 Compass Navigator
Equip players with a special compass that simplifies server navigation.

- **Interactive Interface:**
  - **Compass Menu:** Opens a form displaying all available servers.
  - **Live Status:** Shows server status and current player counts.
  - **Direct Queue Access:** Join the queue just like interacting with NPCs.

### 👑 Rank System Integration
Seamlessly integrate with Discord roles to enhance the rank system across the hub.

- **Asynchronous Rank Loading ⚡**
  - **Non-Blocking API:** Load player ranks in the background without affecting performance.
  - **Instant Recognition:** Cached ranks ensure returning players are quickly recognized.
  - **Background Refresh:** Ranks are updated continually to stay current.

- **Visual Rank Display 🎨**
  - **Custom Nametags:** Player names showcase their rank with unique styles.
  - **Formatted Chat:** Chat messages reflect rank-specific formatting.
  - **Distinct Styling:** Different colors and fonts for Admins, Moderators, Premium members, etc.

### 🎨 Resource Pack Management
Ensure the hub always uses the latest version of the [PokeBedrock Resource Pack](https://github.com/smell-of-curry/pokebedrock-res) for a consistent experience.

- **Version Checking 🔍**
  - **Automatic Updates:** Detects and applies new resource pack versions from GitHub releases.
  - **Integrity Verification:** Ensures the resource pack is current and valid.
  
- **Asset Management 📦**
  - **Dynamic Organization:** Unpacks and organizes assets on the fly.
  - **Custom Support:** Fully supports textures, models, and sounds.
  - **Efficient Loading:** Loads only the necessary resources for optimal performance.

### 🔒 Moderation Integration

**Purpose:**  
Enable seamless interaction with the external moderation system on `pokebedrock.com`.

- **API Communication 🔗**
  - **Data Sync:** Send and receive moderation data (bans, mutes, freezes, warnings) via the API.
  
- **Operator Commands 💬**
  - **Custom Commands:** Implement commands like ban, mute, and kick to update user statuses through the API.

For further details, see the [Moderation System documentation](./docs/ModerationSystem.md).

## Additional Notes 📝

- **Player Data:** Only staff and players with special ranks have persistent data.
- **Server Count Display:** A combined player count across all network servers is showcased.
- **World Boundaries:** The hub world is finite with custom limits to prevent endless chunk generation.
- **Localization:** All player-facing text is translated via the resource pack to support multiple languages.
