<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Icon from "@/lib/Icon.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type FilterMode = "status" | "tier";

	let filterMode: FilterMode = $state("status");
	let enabledStatuses: WatchedStatus[] = $state(["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"]);
	let tiers: Tier[] = $state([]);
	let enabledTierIds: number[] = $state([]);
	let tiersLoaded = $state(false);
	let statusCounts: Record<string, number> = $state({});

	let allItems: Media[] = $state([]);
	let loading = $state(true);
	let error = $state("");
	let gamePhase: "setup" | "playing" | "gameover" = $state("setup");

	let options: { item: Media; genres: string[] }[] = $state([]);
	let oddIndex = $state(-1);
	let sharedGenre = $state("");
	let selectedIndex = $state(-1);
	let answered = $state(false);
	let score = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let round = $state(0);
	let usedKeys = new Set<string>();

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) { if (enabledStatuses.length <= 1) return; enabledStatuses = enabledStatuses.filter((x) => x !== s); }
		else { enabledStatuses = [...enabledStatuses, s]; }
	}
	async function loadTiers() {
		if (tiersLoaded) return;
		try { const r = await axios.get("/tierlist"); tiers = r.data || []; tiersLoaded = true; if (tiers.length > 0) enabledTierIds = tiers.map((t) => t.id); }
		catch { tiers = []; tiersLoaded = true; }
	}
	function toggleTier(id: number) {
		if (enabledTierIds.includes(id)) { if (enabledTierIds.length <= 1) return; enabledTierIds = enabledTierIds.filter((x) => x !== id); }
		else { enabledTierIds = [...enabledTierIds, id]; }
	}
	function switchFilterMode(m: FilterMode) { if (filterMode === m) return; filterMode = m; if (m === "tier") loadTiers(); }

	async function fetchStatusCounts() {
		const statuses: WatchedStatus[] = ["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"];
		const counts: Record<string, number> = {};
		await Promise.all(statuses.map(async (s) => {
			try { const r = await axios.get("/watched", { params: { status: s, limit: 1, page: 1 } }); counts[s] = r.data?.totalResults ?? 0; }
			catch { counts[s] = 0; }
		}));
		statusCounts = counts;
	}

	async function loadItems() {
		loading = true; error = "";
		try {
			if (filterMode === "tier") {
				await loadTiers();
				const ids = new Set<number>();
				for (const t of tiers) { if (enabledTierIds.includes(t.id)) for (const ti of t.tierItems ?? []) ids.add(ti.watchedId); }
				if (ids.size === 0) { allItems = []; loading = false; return; }
				const r = await axios.get("/watched", { params: { limit: 500, page: 1 } });
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name && m.watched && ids.has(m.watched.id));
			} else {
				const r = await axios.get("/watched", { params: { status: enabledStatuses.join(","), limit: 500, page: 1 } });
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name);
			}
		} catch { error = "Failed to load items."; allItems = []; }
		loading = false;
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr]; for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a;
	}

	function getContentType(m: Media): string {
		if (m.type === MediaTypeE.tmdbMovie) return "movie";
		if (m.type === MediaTypeE.tmdbShow) return "tv";
		if (m.type === MediaTypeE.igdbGame) return "game";
		return "movie";
	}

	function getKey(m: Media): string {
		return `${m.type}-${m.ids?.tmdbId ?? m.ids?.igdbId ?? m.name}`;
	}

	async function fetchDetails(m: Media): Promise<any | null> {
		try {
			const ct = getContentType(m);
			const id = ct === "game" ? m.ids?.igdbId : m.ids?.tmdbId;
			if (!id) return null;
			const r = await axios.get(`/content/${ct}/${id}`);
			return r.data;
		} catch { return null; }
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w300${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) return `https://images.igdb.com/igdb/image/upload/t_cover_big/${m.extPosterPath}.jpg`;
		if (m.type === MediaTypeE.malManga) return m.extPosterPath;
		return "";
	}

	async function setupRound(): Promise<boolean> {
		let pool = allItems.filter((m) => !usedKeys.has(getKey(m)));
		if (pool.length < 4) { usedKeys.clear(); pool = [...allItems]; }
		if (pool.length < 4) return false;

		const shuffled = shuffle(pool);
		// Find items with genres
		const withGenres: { item: Media; genres: string[] }[] = [];
		for (const m of shuffled) {
			if (withGenres.length >= 20) break;
			const detail = await fetchDetails(m);
			const genres: string[] = (detail?.genres ?? []).map((g: any) => typeof g === "string" ? g : g.name).filter(Boolean);
			if (genres.length > 0) withGenres.push({ item: m, genres });
		}

		if (withGenres.length < 4) return false;

		// Find a group of 3 sharing a genre + 1 that doesn't have it
		const genreMap: Map<string, typeof withGenres> = new Map();
		for (const wg of withGenres) {
			for (const g of wg.genres) {
				if (!genreMap.has(g)) genreMap.set(g, []);
				genreMap.get(g)!.push(wg);
			}
		}

		// Shuffle genres to be random
		const genreEntries = shuffle([...genreMap.entries()]);
		for (const [genre, items] of genreEntries) {
			if (items.length < 3) continue;
			const group = shuffle(items).slice(0, 3);
			const groupKeys = new Set(group.map((g) => getKey(g.item)));
			// Find an item that does NOT have this genre
			const odd = withGenres.find((wg) => !wg.genres.includes(genre) && !groupKeys.has(getKey(wg.item)));
			if (!odd) continue;

			// Build the round
			const insertPos = Math.floor(Math.random() * 4);
			const result: typeof withGenres = [];
			let gi = 0;
			for (let i = 0; i < 4; i++) {
				if (i === insertPos) result.push(odd);
				else result.push(group[gi++]);
			}

			options = result;
			oddIndex = insertPos;
			sharedGenre = genre;
			selectedIndex = -1;
			answered = false;

			for (const o of result) usedKeys.add(getKey(o.item));
			return true;
		}

		return false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) { error = "You need at least 4 items with genres."; return; }
		error = ""; score = 0; streak = 0; bestStreak = 0; round = 0;
		usedKeys.clear();
		if (!(await setupRound())) { error = "Couldn't find items with different genres."; return; }
		gamePhase = "playing";
	}

	async function selectOption(idx: number) {
		if (answered) return;
		selectedIndex = idx;
		answered = true;

		if (idx === oddIndex) {
			const pts = 100 + streak * 25;
			score += pts;
			streak++;
			if (streak > bestStreak) bestStreak = streak;
			round++;
			setTimeout(async () => {
				if (!(await setupRound())) gamePhase = "gameover";
			}, 1500);
		} else {
			streak = 0;
			round++;
			setTimeout(() => { gamePhase = "gameover"; }, 1500);
		}
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			axios.post("/gamescore", { game: "oddone", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Odd One Out</title></svelte:head>

<div class="oddone-page">
	<PageTitle title="Odd One Out" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Three items share a genre. Find the one that doesn't belong!</p>

		<div class="filter-mode-toggle">
			<button class="plain filter-mode-btn" class:active={filterMode === "status"} onclick={() => switchFilterMode("status")}>Status</button>
			<button class="plain filter-mode-btn" class:active={filterMode === "tier"} onclick={() => switchFilterMode("tier")}>Tierlist</button>
		</div>
		{#if filterMode === "status"}
			<div class="picker-filters">
				{#each [["PLANNED", "Planned"], ["WATCHING", "Watching"], ["HOLD", "On Hold"], ["FINISHED", "Finished"], ["DROPPED", "Dropped"]] as [s, label]}
					<button class="plain filter-btn" class:active={enabledStatuses.includes(s as WatchedStatus)} onclick={() => toggleStatus(s as WatchedStatus)}>
						{label}{statusCounts[s] != null ? ` (${statusCounts[s]})` : ""}
					</button>
				{/each}
			</div>
		{:else}
			<div class="picker-filters">
				{#if !tiersLoaded}<SpinnerTiny />
				{:else if tiers.length === 0}<span class="no-tiers">No tiers found.</span>
				{:else}
					{#each tiers as tier}
						<button class="plain filter-btn tier-filter-btn" class:active={enabledTierIds.includes(tier.id)} onclick={() => toggleTier(tier.id)} style="--tier-bg: {tier.color}; --tier-text: {tier.textColor};">{tier.name}</button>
					{/each}
				{/if}
			</div>
		{/if}

		{#if error}<div class="error-msg">{error}</div>{/if}
		<button class="plain start-btn" onclick={startGame} disabled={loading}>
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="search" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		<p class="hint-text">Which one doesn't share a genre with the others?</p>

		<div class="options-grid">
			{#each options as opt, i}
				{@const poster = getPoster(opt.item)}
				{@const isOdd = i === oddIndex}
				{@const isSelected = i === selectedIndex}
				<button
					class="plain option-card"
					class:correct={answered && isOdd}
					class:wrong={answered && isSelected && !isOdd}
					class:dimmed={answered && !isOdd && !isSelected}
					onclick={() => selectOption(i)}
					disabled={answered}
				>
					{#if poster}<img src={poster} alt="" class="option-poster" />{/if}
					<span class="option-name">{opt.item.name}</span>
					{#if answered}
						<div class="genre-tags">
							{#each opt.genres as g}
								<span class="genre-tag" class:shared={g === sharedGenre}>{g}</span>
							{/each}
						</div>
					{/if}
					{#if answered && isOdd}
						<span class="badge-odd">Odd One!</span>
					{/if}
				</button>
			{/each}
		</div>

		{#if answered}
			<div class="round-feedback">
				{#if selectedIndex === oddIndex}
					<span class="feedback-correct">✅ Correct! +{100 + (streak - 1) * 25}</span>
				{:else}
					<span class="feedback-wrong">❌ Wrong! Game Over</span>
				{/if}
				<span class="feedback-genre">Shared genre: <strong>{sharedGenre}</strong></span>
			</div>
		{/if}

	{:else if gamePhase === "gameover"}
		<div class="result-card">
			<div class="result-emoji">{score > 500 ? "🏆" : score > 200 ? "🌟" : score > 0 ? "👍" : "💀"}</div>
			<h2>Game Over!</h2>
			<div class="result-stats">
				<div class="result-stat"><span class="result-stat-value">{score.toLocaleString()}</span><span class="result-stat-label">Final Score</span></div>
				<div class="result-stat"><span class="result-stat-value">{round}</span><span class="result-stat-label">Rounds Survived</span></div>
				<div class="result-stat"><span class="result-stat-value">🔥 {bestStreak}</span><span class="result-stat-label">Best Streak</span></div>
			</div>
			<div class="result-actions">
				<button class="plain start-btn" onclick={startGame}><Icon i="refresh" wh={18} /> Play Again</button>
				<a href="/games" class="plain back-link">Back to Games</a>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
	.oddone-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 600px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; text-align: center; }
	.filter-mode-toggle { display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px; }
	.filter-mode-btn { padding: 6px 14px; border-radius: 8px; border: none; background: transparent; color: $text-color-accent; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; } &:hover:not(.active) { color: $text-color; } }
	.picker-filters { display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; }
	.filter-btn { padding: 6px 12px; border-radius: 8px; border: 1px solid $bg-color-accent; background: transparent; color: $text-color-accent; font-size: 12px; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; border-color: $accent-color-hover; } }
	.error-msg { color: #ff6b6b; font-size: 14px; }
	.start-btn { display: flex; align-items: center; gap: 8px; padding: 12px 28px; border-radius: 12px; background: $accent-color-hover; color: $bg-color; fill: $bg-color; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 150ms ease, opacity 150ms ease; &:hover { transform: scale(1.03); } &:disabled { opacity: 0.5; cursor: not-allowed; } }
	.game-header { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 11px; color: $text-color-accent; font-weight: 600; text-transform: uppercase; }
	.stat-value { font-size: 20px; font-weight: 700; }
	.hint-text { font-size: 16px; color: $text-color-accent; margin: 0; text-align: center; font-weight: 500; }
	.options-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; width: 100%; }
	.option-card { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px; border-radius: 12px; background: $accent-color; border: 2px solid transparent; cursor: pointer; transition: all 200ms ease;
		&:hover:not(:disabled) { border-color: $accent-color-hover; transform: scale(1.02); }
		&.correct { border-color: #51cf66; background: rgba(81,207,102,0.15); }
		&.wrong { border-color: #ff6b6b; background: rgba(255,107,107,0.15); }
		&.dimmed { opacity: 0.5; }
		&:disabled { cursor: default; }
	}
	.option-poster { width: 100%; max-width: 120px; height: 160px; object-fit: cover; border-radius: 8px; }
	.option-name { font-size: 13px; font-weight: 600; text-align: center; word-break: break-word; }
	.genre-tags { display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
	.genre-tag { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: $bg-color-accent; color: $text-color-accent; &.shared { background: rgba(81,207,102,0.3); color: #51cf66; font-weight: 600; } }
	.badge-odd { font-size: 12px; font-weight: 700; color: #ff6b6b; background: rgba(255,107,107,0.2); padding: 4px 10px; border-radius: 6px; }
	.round-feedback { display: flex; flex-direction: column; align-items: center; gap: 6px; }
	.feedback-correct { font-size: 18px; font-weight: 700; color: #51cf66; }
	.feedback-wrong { font-size: 18px; font-weight: 700; color: #ff6b6b; }
	.feedback-genre { font-size: 14px; color: $text-color-accent; }
	.result-card { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 32px; border-radius: 16px; background: $accent-color; border: 1px solid $bg-color-accent; }
	.result-emoji { font-size: 64px; animation: result-bounce 500ms ease; }
	@keyframes result-bounce { 0% { transform: scale(0); } 60% { transform: scale(1.3); } 100% { transform: scale(1); } }
	.result-stats { display: flex; gap: 24px; flex-wrap: wrap; justify-content: center; }
	.result-stat { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.result-stat-value { font-size: 24px; font-weight: 700; color: $text-color; }
	.result-stat-label { font-size: 12px; color: $text-color-accent; font-weight: 600; }
	.result-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
	.back-link { color: $text-color-accent; font-size: 14px; text-decoration: none; &:hover { color: $text-color; } }
</style>
