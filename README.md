# PokeBedrock Server Hub

The PokeBedrock Server Hub is a powerful, high-performance central lobby system built with [Dragonfly](https://github.com/df-mc/dragonfly). It serves as the entry point for players into the PokeBedrock network, managing server connections, player queues, and providing a seamless experience for navigating between the main servers.

## Features

### NPC Slapper System

The hub employs interactive NPCs throughout the world that allow players to connect to various servers in the network.

- **Dynamic Nametags**:
  - Real-time updates showing current online player counts
  - Server status indicators (online/offline)
  - Visual indicators of server capacity

- **Smart Interaction**:
  - Players can interact with NPCs to initiate server transfer
  - Confirmation dialog prevents accidental transfers
  - Automatic queue placement when servers are at capacity

### Priority Queue System

The hub features a sophisticated priority queue system that manages player transfers efficiently, especially during high-traffic periods.

- **Visual Queue Display**:
  - Boss bar shows accurate queue position, waiting time, and destination
  - Position updates in real-time based on priority algorithms
  - Clear indication of estimated wait time

- **Priority Rankings**:
  - Players are sorted based on their rank (Admin > Moderator > Premium > Trainer)
  - Within the same rank, players are ordered by join time (first come, first served)
  - Queue position is clearly communicated to players

- **Intelligent Transfer**:
  - Players are transferred automatically when space becomes available
  - Failed transfers are handled gracefully with automatic queue re-entry
  - Admin bypass allows staff to join near-capacity servers

### Compass Navigator
  - Players receive a special compass in their inventory
  - Compass opens a form showing all available servers
  - Server status and player counts are displayed in the navigation interface
  - Allows players to enter into the queue for the server like the npcs.

### Rank System Integration

The hub integrates with Discord roles to provide a comprehensive rank system.

- **Asynchronous Rank Loading**:
  - Player ranks are loaded from the API without blocking server performance
  - Cached ranks provide instant recognition for returning players
  - Background refresh ensures ranks stay current

- **Visual Rank Display**:
  - Custom formatted nametags show player ranks
  - Chat messages are formatted according to player rank
  - Distinct colors and styles for different ranks (Admin, Moderator, Premium, etc.)

### Resource Pack Management

Since the PokeBedrock Hub requires the use of the [PokeBedrock Resource Pack](https://github.com/smell-of-curry/pokebedrock-res) it must always make sure that it is using the current, up to date, version.

- **Version Checking**:
  - Automatic detection of new resource pack versions
  - Seamless updates from GitHub releases
  - Verification of pack integrity

- **Asset Management**:
  - Dynamic unpacking and organization of resource assets
  - Support for custom textures, models, and sounds
  - Efficient loading of only necessary resources

## 4. Moderation Integration

**Purpose:**  
Enable the hub to interact with the external moderation system on `pokebedrock.com`.

**Requirements:**

- **API Communication:**
  - The hub must be able to send and receive data from the moderation system via its API.
  - The API provides moderation details such as bans, mutes, freezes, and warnings.

- **Operator Commands:**
  - Implement custom commands (e.g., ban, mute, kick) that communicate with the moderation API to update user statuses.

The Moderation system details can be found [here](./docs/ModerationSystem.md)

## Additional Notes

- **Player Data**: The hub only persists data for staff members and players with special ranks
- **Server Count Display**: The hub displays a combined player count across all network servers
- **World Limitations**: The world is finite with custom boundaries to prevent chunk generation
- **Localization**: All player-facing text uses resource pack translations for multi-language support