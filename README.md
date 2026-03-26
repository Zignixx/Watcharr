<h1 align="center">Watcharr</h1>
<p align="center"><img src="./static/logo-col.png" alt="logo" width="250" /></p>

<p align="center">
  <a href="https://github.com/sbondCo/Watcharr/pkgs/container/watcharr"><img src="https://img.shields.io/github/v/release/sbondCo/Watcharr?label=version&style=for-the-badge" /></a>
  <a href="https://beta.watcharr.app"><img src="https://img.shields.io/website?label=DEMO&style=for-the-badge&url=https%3A%2F%2Fbeta.watcharr.app" /></a>
  <a href="https://watcharr.app"><img src="https://img.shields.io/website?label=DOCS&style=for-the-badge&url=https%3A%2F%2Fwatcharr.app" /></a>
  <a href="https://github.com/sbondCo/Watcharr/issues"><img src="https://img.shields.io/github/issues-raw/sbondCo/Watcharr?label=ISSUES&style=for-the-badge" /></a>
  <a href="/LICENSE"><img src="https://img.shields.io/github/license/sbondCo/Watcharr?style=for-the-badge" /></a>
  <a href="https://matrix.to/#/#watcharr:matrix.org"><img src="https://img.shields.io/matrix/watcharr%3Amatrix.org?style=for-the-badge&logo=matrix" /></a>
</p>

I'm your new easily self-hosted content watched list. The place you store your watched (or watching, planned, etc) **movies** and **tv shows** (and **anime**), rate them and track their status.

With [some extra configuration](https://watcharr.app/docs/server_config/game-support-igdb) I can also track your **video games**.

I am built with Go and Svelte(Kit).

Feel free to abuse this demo instance (nicely), which runs on the latest `dev` build (there may be bugs, as new features are tested on here too): [https://beta.watcharr.app/](https://beta.watcharr.app/)

[Track progress for the next version](https://github.com/orgs/sbondCo/projects/9/views/3).

### Contents

- [Fork Changes (by Zignixx)](#fork-changes-by-zignixx)
- [Screenshots](#screenshots)
- [Set Up](#set-up)
- [Community Made Tools](#community-made-tools)
- [Getting Help](#getting-help)
- [Contributing](#contributing)

---

# Fork Changes (by Zignixx)

> **Note:** All features listed below were developed through **Vibecode** with **Claude Opus 4.6** (Anthropic).

This fork extends the original Watcharr with a wide range of new features, UI improvements, and quality-of-life enhancements. Below is a detailed breakdown of everything that was added or changed.

## � Tierlist System

A full tierlist feature was added from scratch — both backend (database entities, API routes) and frontend.

- **Drag & Drop tier management**: Create custom tiers, reorder them, drag content items between tiers
- **20+ color gradient presets**: Classic, Sunset, Ocean, Neon, Forest, Aurora, Galaxy and more — applied automatically across tiers
- **Gradient options**: Reverse gradient direction, custom text color override
- **Preset management**: Save and load custom tier templates
- **Rating sync**: Sync tier placement with your rating system
- **Status filtering**: Filter untiered items by watch status
- **PNG export**: Export your tierlist as a PNG image with user watermark
- **Public tierlists**: Share your tierlist publicly via `/lists/[userId]/[username]` (respects privacy settings)
- **Context menu on tierlists**: Right-click tiers for quick actions

## 🎮 Mini Games Hub

A full mini games section (`/games`) was added, with persistent high score tracking (backend + frontend). All games support **status filtering** (Planned, Watching, Finished, etc.) and **tierlist filtering**. Results in every game are **skippable** (click to skip the result delay, with 300ms minimum to prevent accidental skips).

### Trivia Quiz
- 15 questions per round with prize ladder labels (100 – 1.000.000)
- Question types: Genre, Poster (blurred at 25px), Runtime, Seasons, Episode Count, Summary/Description (every other word blurred), Media Type, Anime identification, and Comparison questions (Which has more seasons? Which has the longest runtime?)
- Difficulty tiers: easy → medium → hard (based on question position)
- No two consecutive questions of the same category
- Lifelines: 50:50 and Skip
- Score: base points + streak bonus

### Higher or Lower
- Three compare modes: **Rating**, **Release Year**, and **Total Episodes**
- Episodes mode automatically filters to TV shows only
- Streak-based scoring, shift-left mechanic (correct answer slides to the left card)

### Guess the Poster
- Progressive blur difficulty: Rounds 0–2 → 10px blur, Rounds 3–5 → 20px blur, Rounds 6+ → 30px blur
- 1 guess per round, endless mode
- 4 options per round

### Plot Twist
- Shows a media description word-by-word with a 10-second countdown timer
- Even-indexed words are revealed one at a time (~400ms interval)
- Odd-indexed words are permanently blurred (visible but unreadable)
- Speed bonus scoring based on remaining time
- Timer bar turns red when ≤3 seconds remain
- After answering or timeout, all words are fully revealed

### Release Timeline
- Drag & drop items into the correct chronological order by release year
- Live reordering with scale/shadow effects, pop/shake animations
- Scoring: 200 + streak × 50

### Name That Show
- Shows 2–3 episode titles from a random season of a TV show
- Guess the correct show from 4 options
- Only TV shows are used

### Episode Counter
- Shows total episode count and number of seasons
- Guess the correct TV show from 4 options

### Game Score Persistence
- Backend API: `POST /api/gamescore` and `GET /api/gamescore`
- Stores score, best streak, and times played per game per user
- Games hub shows 🏆 High Score, 🔥 Best Streak, and 🎮 Times Played for each game

## 🎲 Random Picker
- Wheel-of-fortune style random picker for your watchlist
- Filter by status or tierlist tiers
- Audio feedback (Roll.mp3) during spinning

## 📺 Manga Support
- Full manga tracking support (MAL-based) with dedicated manga detail page
- Manga-specific metadata: status, chapter count, volume count, authors
- Manage watch status, rating, thoughts, and pinned status directly from the manga page
- Manga discovery via Jikan API integration
- Manga search integrated into the global search with media type filter

## 🧭 New Navigation & Detail Menu
- **Detailed Menu** (`DetailedMenu`): Dropdown to toggle which metadata appears on poster items — Title, Status Color, Status & Rating, Watching Season/Chapter, Date Added, Date Modified
- New layout navigation entries for Tierlist, Picker, Games, and Users pages

## 📋 List View & Grid View Toggle
- **List View**: A searchable table-based view with multi-column sorting (Name, Type, Year, Status, Rating, Date Added)
- Toggle between traditional poster grid view and tabular list view
- Owner watched data visible on shared lists (see what the list owner rated/status'd)

## 🖱️ Poster Context Menu
- Right-click any poster to open a context menu
- Quickly change watch status (Planned / Watching / Finished / Hold / Dropped)
- Add/edit personal ratings and thoughts inline
- View comparisons with followed users
- Delete item from your list

## 📦 Import & Export
- **Text Import Modal**: Bulk import from plain text lists with year parsing (e.g. "The Dark Knight (2008)"), select media type (Movie/TV/Game), set global watch status, supports Movary-style CSV imports (history, ratings, watchlist)
- **Export Modal**: Export your list as CSV, JSON, or plain text with live preview (first 2000 chars), download as timestamped file or copy to clipboard — includes Name, Type, Year, Status, Rating, Date Added, Thoughts

## 👥 Social & Follow System
- **Follow users**: Follow other users to see their activity
- **Followed Thoughts**: On each content page, see what users you follow have watched, rated, or planned that item — grouped by status
- **Social Badge on Posters**: Small badge appears on posters when followed users have rated/watched that item, expanding to show usernames and their status on click
- **Public User List** (`/users`): Browsable grid of all non-private users with avatars, usernames, and bios — click to visit their watchlist

## 🔍 Enhanced Recommendations
- Personalized movie and show recommendations based on your watched list
- **Recommendation Source**: Shows *why* an item was recommended (which of your watched items triggered it)
- Configurable parameters and improved caching system
- Real-time progress polling: see how many recommendations have been computed
- Refresh button to force recalculation, with cache indicator in the UI

## 📱 Mobile & Touch Improvements
- **PosterGestureHUD**: Full-screen touch gesture overlay for quick rating/status changes
  - Swipe vertically to rate (supports all rating systems: 10-point, 5-point, 100-point, thumbs, and tierlist)
  - Swipe horizontally to change status
  - Cancel zone at the bottom to abort
  - Blurred poster background with animated feedback
- **Mobile Poster Modal**: Enhanced modal for media interaction on mobile
- Improved responsive design and layout adjustments across the app

## 👤 Profile Enhancements
- **Time format cycling**: Click on watch-time stats to cycle through 7 formats (auto / minutes / hours / days / weeks / months / years)
- Comprehensive viewing stats: movies/shows watched, total runtime, planned content runtime
- Editable bio and custom avatar upload
- Theme selection (System / Light / Dark)

## 🔧 Admin & User Management
- **Edit User Modal**: Admins can rename users, modify permissions (Admin, Request Content, Auto-Approve Requests), change user type (Watcharr/Proxy), and delete users with confirmation dialogs
- Enhanced input fields with save buttons for server configuration page

## 🎨 UI & Other Improvements
- Poster readiness state with improved visibility transitions
- Hover and focus styles for view toggle and export buttons
- Title detail added to ExtraDetails component
- Interaction toggling for media items based on login status
- Various code refactors and quality-of-life fixes

# Screenshots

<p align="center">

<img src="./screenshot/devices-mock.png" alt="Overview" />

| Homepage                                                       | Watched Show Hover                                                          |
| -------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="./screenshot/homepage.png?v=2" alt="Watched List" /> | <img src="./screenshot/homepage-poster-hover.png?v=2" alt="Watched List" /> |

| Watched Show Status Change                                                                  | Movie Details                                                                      |
| ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| <img src="./screenshot/homepage-poster-change-status.png?v=2" alt="Changing Show Status" /> | <img src="./screenshot/content-details-page.png?v=2" alt="Content Details Page" /> |

| User Profile                                                            | Discover                                                             |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------- |
| <img src="./screenshot/user-profile.png?v=2" alt="User Profile Page" /> | <img src="./screenshot/discover-page.png?v=2" alt="Discover Page" /> |

| Dark Homepage                                                              | Dark Content Details                                                                       |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| <img src="./screenshot/homepage-dark.png?v=2" alt="Dark Theme Homepage" /> | <img src="./screenshot/content-details-page-dark.png?v=2" alt="Dark Theme Content Page" /> |

</p>

# Set Up

[Checkout our documentation](https://watcharr.app/docs/category/installation) for an up to date guide on setup! If you hate manuals, but love docker, this [docker-compose.yml](./docker-compose.yml) file is your friend.

# Community Made Tools

Third-party tools made by the community for enhancing your Watcharr experience!

- [Kodi Plugin](https://github.com/airdogvan/watcharr_kodi) by [airdogvan](https://github.com/airdogvan) for automatically tracking your watched shows/movies.

Thanks to anyone that has made a script or tool for Watcharr. Feel free to add your own to the list if you have one!

**Note:** I cannot provide any assurances for these tools or stay on top of them (code review, etc), if you have any problems please open an issue in the project for the tool so that they can stay organized.

# Getting Help

If something isn't working for you or you are stuck, [creating an issue](https://github.com/sbondCo/Watcharr/issues/new) is the best way to get help! Every type of issue is accepted, so don't be afraid to ask anything!

You can also [join our space on Matrix](https://matrix.to/#/#watcharr:matrix.org) for support.

# License

This project is licensed under the GPLv3 license. You should see the [LICENSE](LICENSE) file located in the root folder of this project for the full license text, if not, see <https://www.gnu.org/licenses/>.

# Contributing

Please continue to our [contributing guide](CONTRIBUTING.md).
