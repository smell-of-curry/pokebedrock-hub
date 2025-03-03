# PokeBedrock Server Hub 

This project involves remaking the current server hub (coded in [PocketMine](https://github.com/pmmp/PocketMine-MP)) and porting it over to [Dragonfly](https://github.com/df-mc/dragonfly). Although the project is not overly complex, it requires several specific features. 

Below is a detailed list of requirements.

## 1. NPC Slapper System

**Purpose:**  
Allow players to click on in-world NPCs to transfer to another server.

**Requirements:**
- **Spawning & Configuration:**
  - When an NPC is spawned, a modal form should be displayed that lets the user modify the target address (default format: `IP:PORT`, e.g., `us.pokebedrock.com:19132`).
  - Allow editing of the NPC’s skin ID. The mapping reference is available in the JSON file here: [hub_npc.json](https://github.com/smell-of-curry/pokebedrock-res/blob/main/entity/hub_npc.json).
  - The ability to set a custom name for the server for example: `§fPoke§rBedrock §eGold§r`
  - This form should be able to be open again in the case that something needs to be edited. But only operators will be able to open it by crouching and right clicking.

- **Dynamic Nametag:**
  The NPC’s nametag should update in real time to show:
  - The number of current online users.
  - The server’s current status (e.g., online/offline).
  - The current ping of the user to that server (if applicable).

- **Interaction & Transfer:**
  - When a player interacts (hits) the NPC, display a confirmation screen.
  - The confirmation screen should inform the player that they are about to transfer to a new server.
  - If the target server is full, or there is players in the queue, the system should allow the player to join the queue (see Queue System below); otherwise, provide an immediate transfer option via a button.

## 2. Queue System

**Purpose:**  
Manage player transfers when a target server is at capacity.

**Requirements:**

- **Queue Display:**
  Use the boss bar to show queue status.
  The boss bar should display:
  - The player’s current position in the queue.
  - The total time the player has been waiting.
  - The destination server’s name.

- **Priority Queue**
  - The queue also has a priority queue system, where as if a staff or a user who should have priority queue joins the queue
  they are automatically pushed up above all non-priority members, still keeping position of others in priority queue.
  - When a user joins the priority queue, all users should be moved down in position and notified through a message.
  - The following roles get Priority Queue:
    - Operators
    - Administrators (discord)
    - Supporters (discord)
    - Server Boosters (discord)

- **Security & Future Enhancements:**
  - Note that while the hub will display queue information, the secure transmission of join requests to the upstream server will be implemented at a later stage.
  - This will just require the Hub to send a Handshake request to the upstream [Server Proxy](https://github.com/smell-of-curry/vanilla-proxy) informing that it is about to authorize a user to join.
  - Each user MUST get this validation from the queue, else they will just be transferred back to the hub.

## 3. Compass System

**Purpose:**  
Offer an alternative method for transferring to different servers using a compass tool.

**Requirements:**

- **Interaction:**
  - Instead of interacting with an NPC, players can use the compass to trigger an Action Form.
  - The Action Form should list available servers for transfer.

- **Icons & Slot:**
  - Each server in the list should display its designated icon (as provided in the resource pack).
  - The compass should be:
    - Automatically given to players on spawn.
    - Locked in the 9th slot of the player’s hotbar.
    - Non-droppable by players.

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

## 5. Chat Ranks System

**Purpose:**  
Allow chat messages to include user ranks (e.g., `[OWNER]`, `[ADMIN]`).

**Requirements:**

- **Dynamic Rank Assignment:**
  - Implement a system that can assign and change chat ranks dynamically (preferably a form, but commands work too). 
  - The system should update in real time and reflect in the chat output.
  - The users should have there rank displayed below there name something like: `Smell of curry\n[§cOwner§r]`

## 6. Resource Pack Auto-Fetch
Since the PokeBedrock Hub requires the use of the [PokeBedrock Resource Pack](https://github.com/smell-of-curry/pokebedrock-res) it must always make sure that it is using the current, up to date, version.

Therefore once the application is run, and before the server starts, a prompt should be added in terminal to ask the administrator to update the current resource pack to the new version if available. This can be denied but will be always asked again next initialization.

This will be created by fetching: `https://api.github.com/repos/smell-of-curry/pokebedrock-res/releases/latest`, comparing it to the current installed version, and checking if they match.

## 7. Miscellaneous Requirements

**Player Data:**

- Non-staff player data should not be saved. Only staff (or players with custom chat ranks) will have persistent data.

**Server Count Display:**

- The hub’s displayed player count should be a combined total of:
  - The number of players currently on the hub.
  - The total number of players across all linked servers.
  
  _For example:_
  - If there are 20 players on server White, 30 on server Black, and 2 on the hub, the displayed count should be **52**.

**Finite World Size**
- The server should be custom fit to the world provided, ensuring that no new chunks can ever be loaded or any custom mob spawning.

**All Translatable**
- All text that is displayed to the user (except for user inputted details from the NPC Forms) should be using translatable text using the resource packs .lang. So for example instead of saying "Your position in queue is x" it should use `rawtext` to use a translatable string `hub.queue.position` and pass in the value.