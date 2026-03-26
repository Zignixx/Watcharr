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
	let gamePhase: "setup" | "playing" | "result" = $state("setup");

	let currentItem: Media | null = $state(null);
	let actualRating = $state(0);
	let userGuess = $state(5.0);
	let submitted = $state(false);
	let score = $state(0);
	let round = $state(0);
	let totalRounds = 10;
	let roundScore = $state(0);
	let totalDiff = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let usedKeys = new Set<string>();
	let avgDiff = $derived(round > 0 ? (totalDiff / round).toFixed(2) : "0");

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
		return `${m.type}-${m.ids?.tmdb ?? m.ids?.igdb ?? m.name}`;
	}

	async function fetchDetails(m: Media): Promise<Media | null> {
		try {
			const ct = getContentType(m);
			const id = ct === "game" ? m.ids?.igdb : m.ids?.tmdb;
			if (!id) return null;
			const r = ct === "game" ? await axios.get(`/game/${id}`) : await axios.get(`/content/${ct}/${id}`);
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
		if (pool.length < 1) { usedKeys.clear(); pool = [...allItems]; }
		if (pool.length < 1) return false;

		const candidates = shuffle(pool);
		for (const m of candidates) {
			const detail = await fetchDetails(m);
			if (!detail || !detail.rating || detail.rating === 0) continue;
			currentItem = m;
			actualRating = detail.rating;
			usedKeys.add(getKey(m));
			userGuess = 5;
			submitted = false;
			roundScore = 0;
			return true;
		}
		return false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 1) { error = "You need at least 1 item."; return; }
		error = ""; score = 0; round = 0; totalDiff = 0; streak = 0; bestStreak = 0;
		usedKeys.clear();
		if (!(await setupRound())) { error = "Couldn't find items with ratings."; return; }
		gamePhase = "playing";
	}

	async function submitGuess() {
		if (submitted) return;
		submitted = true;

		const diff = Math.abs(userGuess - actualRating);
		const maxPoints = 500;
		// Score: closer = more points. Perfect = 500, off by 5+ = 0
		roundScore = Math.max(0, Math.round(maxPoints * (1 - diff / 5)));
		totalDiff += diff;
		score += roundScore;

		if (diff <= 1) {
			streak++;
			if (streak > bestStreak) bestStreak = streak;
		} else {
			streak = 0;
		}

		setTimeout(async () => {
			round++;
			if (round >= totalRounds || !(await setupRound())) {
				gamePhase = "result";
			}
		}, 2500);
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "result") {
			axios.post("/gamescore", { game: "rating", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Rating Guess</title></svelte:head>

<div class="rating-page">
	<PageTitle title="Rating Guess" />

	{#if gamePhase === "setup"}
		<p class="subtitle">How well do you know the TMDB ratings? Guess the score!</p>

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
						<button class="plain filter-btn tier-filter-btn" class:active={enabledTierIds.includes(tier.id)} onclick={() => toggleTier(tier.id)} style="--tier-bg: {tier.color}; --tier-text: {tier.textColor};">
						{tier.name}{tier.tierItems ? ` (${tier.tierItems.length})` : ""}
					</button>
					{/each}
				{/if}
			</div>
		{/if}

		{#if error}<div class="error-msg">{error}</div>{/if}
		<button class="plain start-btn" onclick={startGame} disabled={loading}>
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="star" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}/{totalRounds}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		{#if currentItem}
			{@const poster = getPoster(currentItem)}
			<div class="item-card">
				{#if poster}<img src={poster} alt="" class="item-poster" />{/if}
				<h2 class="item-name">{currentItem.name}</h2>
			</div>

			<div class="slider-area">
				<div class="slider-labels">
					<span>0</span>
					<span class="guess-display" class:perfect={submitted && userGuess === actualRating} class:close={submitted && Math.abs(userGuess - actualRating) <= 1} class:far={submitted && Math.abs(userGuess - actualRating) > 1}>
						{userGuess}
					</span>
					<span>10</span>
				</div>
				<input
					type="range"
					min="0"
					max="10"
					step="0.1"
					bind:value={userGuess}
					disabled={submitted}
					class="rating-slider"
				/>
			</div>

			{#if !submitted}
				<button class="plain start-btn" onclick={submitGuess}>
					<Icon i="check" wh={18} /> Lock In
				</button>
			{:else}
				<div class="reveal-area">
					<div class="reveal-rating">
						<span class="reveal-label">Actual Rating</span>
						<span class="reveal-value">{actualRating}</span>
					</div>
					<div class="reveal-diff">
						{#if userGuess === actualRating}
							<span class="diff-badge perfect">🎯 Perfect! +{roundScore}</span>
						{:else if Math.abs(userGuess - actualRating) <= 1}
							<span class="diff-badge close">👍 Close! Off by {Math.abs(userGuess - actualRating)} → +{roundScore}</span>
						{:else}
							<span class="diff-badge far">😬 Off by {Math.abs(userGuess - actualRating)} → +{roundScore}</span>
						{/if}
					</div>
				</div>
			{/if}
		{/if}

	{:else if gamePhase === "result"}
		<div class="result-card">
			<div class="result-emoji">{parseFloat(avgDiff) < 0.5 ? "🏆" : parseFloat(avgDiff) < 1 ? "🌟" : parseFloat(avgDiff) < 2 ? "👍" : "💀"}</div>
			<h2>Game Complete!</h2>
			<div class="result-stats">
				<div class="result-stat"><span class="result-stat-value">{score.toLocaleString()}</span><span class="result-stat-label">Total Score</span></div>
				<div class="result-stat"><span class="result-stat-value">{avgDiff}</span><span class="result-stat-label">Avg Difference</span></div>
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
	.rating-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 500px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; text-align: center; }
	.filter-mode-toggle { display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px; }
	.filter-mode-btn { padding: 6px 14px; border-radius: 8px; border: none; background: transparent; color: $text-color-accent; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; } &:hover:not(.active) { color: $text-color; } }
	.picker-filters { display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; }
	.filter-btn { padding: 6px 12px; border-radius: 8px; border: 1px solid $bg-color-accent; background: transparent; color: $text-color-accent; font-size: 12px; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; border-color: $accent-color-hover; } }
	.tier-filter-btn {
		border-color: var(--tier-bg);
		&:hover, &.active { background: var(--tier-bg); color: var(--tier-text); border-color: var(--tier-bg); }
	}
	.error-msg { color: #ff6b6b; font-size: 14px; }
	.start-btn { display: flex; align-items: center; gap: 8px; padding: 12px 28px; border-radius: 12px; background: $accent-color-hover; color: $bg-color; fill: $bg-color; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 150ms ease, opacity 150ms ease; &:hover { transform: scale(1.03); } &:disabled { opacity: 0.5; cursor: not-allowed; } }
	.game-header { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 11px; color: $text-color-accent; font-weight: 600; text-transform: uppercase; }
	.stat-value { font-size: 20px; font-weight: 700; }
	.item-card { display: flex; flex-direction: column; align-items: center; gap: 12px; }
	.item-poster { width: 160px; height: 230px; object-fit: cover; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.3); }
	.item-name { font-size: 20px; font-weight: 700; margin: 0; text-align: center; }
	.slider-area { width: 100%; }
	.slider-labels { display: flex; justify-content: space-between; align-items: center; font-size: 14px; color: $text-color-accent; font-weight: 600; margin-bottom: 8px; }
	.guess-display { font-size: 28px; font-weight: 800; color: $text-color; transition: color 300ms ease; &.perfect { color: #51cf66; } &.close { color: #ffd43b; } &.far { color: #ff6b6b; } }
	.rating-slider { width: 100%; height: 8px; appearance: none; background: $bg-color-accent; border-radius: 4px; outline: none; cursor: pointer;
		&::-webkit-slider-thumb { appearance: none; width: 24px; height: 24px; border-radius: 50%; background: $accent-color-hover; cursor: pointer; }
		&::-moz-range-thumb { width: 24px; height: 24px; border-radius: 50%; background: $accent-color-hover; cursor: pointer; border: none; }
		&:disabled { opacity: 0.6; cursor: default; }
	}
	.reveal-area { display: flex; flex-direction: column; align-items: center; gap: 12px; }
	.reveal-rating { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.reveal-label { font-size: 12px; color: $text-color-accent; font-weight: 600; }
	.reveal-value { font-size: 36px; font-weight: 800; color: #ffd43b; }
	.diff-badge { font-size: 16px; font-weight: 700; padding: 8px 16px; border-radius: 10px; &.perfect { background: rgba(81,207,102,0.2); color: #51cf66; } &.close { background: rgba(255,212,59,0.2); color: #ffd43b; } &.far { background: rgba(255,107,107,0.2); color: #ff6b6b; } }
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
