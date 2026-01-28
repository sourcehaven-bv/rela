# Utility Scripts

Development utility scripts for the rela project.

## tmux-capture

Combined tool for capturing tmux pane screenshots and deduplicating them.

### Usage

```bash
# Capture screenshots from default session
./utils/tmux-capture

# Capture from specific session with custom interval
./utils/tmux-capture -s my-session -i 1.0 capture

# Deduplicate existing screenshots
./utils/tmux-capture dedupe

# Capture and auto-dedupe when stopped (recommended)
./utils/tmux-capture both
```

### Modes

- **capture** - Continuously capture tmux pane screenshots at regular intervals
- **dedupe** - Remove duplicate consecutive screenshots from existing captures
- **both** - Capture screenshots, then automatically deduplicate when stopped (Ctrl+C)

### Options

- `-s SESSION` - Tmux session name (default: rela-demo)
- `-o DIR` - Output directory (default: screendumps)
- `-i INTERVAL` - Capture interval in seconds (default: 0.5)
- `-h` - Show help message

### Examples

**Demo Recording Workflow:**

1. Start your tmux session:

   ```bash
   tmux new -s rela-demo
   ```

2. In another terminal, start capturing:

   ```bash
   ./utils/tmux-capture both
   ```

3. Perform your demo in the tmux session

4. When done, press Ctrl+C in the capture terminal
   - It will automatically deduplicate the screenshots
   - You'll be prompted to renumber files sequentially

**Manual Deduplication:**

If you already have screenshots and just want to deduplicate:

```bash
./utils/tmux-capture dedupe
```

### Output

Screenshots are saved as `screendumps/screen_NNNN.txt` with sequential numbering.

After deduplication, only unique screenshots are kept, significantly reducing the file count while
preserving all unique states.

## Migration from Legacy Scripts

The old separate scripts have been combined:

- `capture-tmux.sh` → `utils/tmux-capture capture`
- `dedupe-screendumps.sh` → `utils/tmux-capture dedupe`

The legacy scripts can be removed after migration.
