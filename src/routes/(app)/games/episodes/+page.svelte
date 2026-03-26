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

	let episodeTitles: string[] = $state([]);
	let seasonLabel = $state("");
	let options: Media[] = $state([]);
	let correctIndex = $state(0);
	let selectedAnswer: number | null = $state(null);
	let answered = $state(false);
	let wasCorrect = $state(false);
	let score = $state(0);
	let round = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let usedKeys = new Set<string>();

	let pendingTimeout: ReturnType<typeof setTimeout> | null = null;
	let pendingAction: (() => void) | null = null;
	let skipReadyAt = 0;
	function scheduleAction(fn: () => void, delay: number) {
		pendingAction = fn;
		pendingTimeout = setTimeout(() => { pendingTimeout = null; pendingAction = null; fn(); }, delay);
		skipReadyAt = Date.now() + 300;
	}
	function skipResult() {
		if (!pendingTimeout || !pendingAction || Date.now() < skipReadyAt) return;
		clearTimeout(pendingTimeout);
		const action = pendingAction;
		pendingTimeout = null; pendingAction = null;
		action();
	}

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
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name && m.type === MediaTypeE.tmdbShow && m.watched && ids.has(m.watched.id));
			} else {
				const r = await axios.get("/watched", { params: { status: enabledStatuses.join(","), limit: 500, page: 1 } });
				allItems = (r.data?.results ?? r.data ?? []).filter((m: Media) => m.name && m.type === MediaTypeE.tmdbShow);
			}
		} catch { error = "Failed to load items."; allItems = []; }
		loading = false;
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr]; for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a;
	}

	function getKey(m: Media): string {
		return `tv-${m.ids?.tmdb ?? m.name}`;
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
		return `https://image.tmdb.org/t/p/w300${m.extPosterPath}`;
	}

	async function fetchEpisodeTitles(tmdbId: number): Promise<{ titles: string[]; seasonLabel: string } | null> {
		try {
			// First get the show detail to find available seasons
			const detail = await axios.get(`/content/tv/${tmdbId}`);
			const seasons: { number: number; episodeCount: number; name: string }[] = detail.data?.seasons ?? [];
			const validSeasons = seasons.filter(s => s.number > 0 && s.episodeCount >= 3);
			if (validSeasons.length === 0) return null;

			// Pick a random season
			const season = validSeasons[Math.floor(Math.random() * validSeasons.length)];
			const seasonData = await axios.get(`/content/tv/${tmdbId}/season/${season.number}`);
			const episodes: { name: string; episode_number: number }[] = seasonData.data?.episodes ?? [];

			if (episodes.length < 3) return null;

			// Pick 2-3 random episodes
			const count = Math.min(3, episodes.length);
			const picked = shuffle(episodes).slice(0, count);
			const titles = picked
				.sort((a, b) => a.episode_number - b.episode_number)
				.map(e => e.name)
				.filter(t => t && t.length > 0);

			if (titles.length < 2) return null;

			return { titles, seasonLabel: `Season ${season.number}` };
		} catch {
			return null;
		}
	}

	async function setupRound(): Promise<boolean> {
		let pool = allItems.filter(m => !usedKeys.has(getKey(m)));
		if (pool.length < 4) { usedKeys.clear(); pool = [...allItems]; }
		if (pool.length < 4) return false;

		const candidates = shuffle(pool);

		for (const candidate of candidates) {
			const tmdbId = candidate.ids?.tmdb;
			if (!tmdbId) continue;

			const result = await fetchEpisodeTitles(tmdbId);
			if (!result) continue;

			usedKeys.add(getKey(candidate));

			// Pick 3 wrong options from the rest
			const others = shuffle(allItems.filter(m => getKey(m) !== getKey(candidate))).slice(0, 3);
			if (others.length < 3) continue;

			const allOptions = shuffle([candidate, ...others]);
			correctIndex = allOptions.findIndex(m => getKey(m) === getKey(candidate));
			options = allOptions;
			episodeTitles = result.titles;
			seasonLabel = result.seasonLabel;
			selectedAnswer = null;
			answered = false;
			wasCorrect = false;
			return true;
		}
		return false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) { error = "You need at least 4 TV shows."; return; }
		error = ""; score = 0; round = 0; streak = 0; bestStreak = 0;
		usedKeys.clear();
		if (!(await setupRound())) { error = "Couldn't find shows with episode data."; return; }
		gamePhase = "playing";
	}

	async function selectAnswer(index: number) {
		if (answered) return;
		selectedAnswer = index;
		answered = true;

		if (index === correctIndex) {
			wasCorrect = true;
			streak++;
			score += 100 + streak * 25;
			if (streak > bestStreak) bestStreak = streak;
			scheduleAction(async () => {
				round++;
				if (!(await setupRound())) gamePhase = "gameover";
			}, 3000);
		} else {
			wasCorrect = false;
			streak = 0;
			scheduleAction(() => { gamePhase = "gameover"; }, 5000);
		}
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			axios.post("/gamescore", { game: "episodes", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Name That Show</title></svelte:head>

<div class="episodes-page" onclick={skipResult}>
	<PageTitle title="Name That Show" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Can you guess the show from its episode titles?</p>

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

		<p class="note">Only TV shows from your watchlist are used.</p>
		{#if error}<div class="error-msg">{error}</div>{/if}
		<button class="plain start-btn" onclick={startGame} disabled={loading}>
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="film" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		<div class="clue-card">
			<span class="clue-label">{seasonLabel} — Episode Titles</span>
			<div class="episode-list">
				{#each episodeTitles as title}
					<div class="episode-title">"{title}"</div>
				{/each}
			</div>
		</div>

		<p class="question-text">Which show do these episodes belong to?</p>

		<div class="options-grid">
			{#each options as opt, i}
				{@const poster = getPoster(opt)}
				{@const isCorrect = i === correctIndex}
				{@const isSelected = i === selectedAnswer}
				<button
					class="plain option-card"
					class:correct={answered && isCorrect}
					class:wrong={answered && isSelected && !isCorrect}
					class:dimmed={answered && !isCorrect && !isSelected}
					onclick={() => selectAnswer(i)}
					disabled={answered}
				>
					{#if poster}<img src={poster} alt="" class="option-poster" />{/if}
					<span class="option-name">{opt.name}</span>
				</button>
			{/each}
		</div>

		{#if answered}
			<div class="round-feedback">
				{#if wasCorrect}
					<span class="feedback-correct">✅ Correct! +{100 + streak * 25}</span>
				{:else}
					<span class="feedback-wrong">❌ Wrong! It was {options[correctIndex]?.name}</span>
				{/if}
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
	.episodes-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 550px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; text-align: center; }
	.note { font-size: 12px; color: $text-color-accent; margin: 0; opacity: 0.7; }
	.filter-mode-toggle { display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px; }
	.filter-mode-btn { padding: 6px 14px; border-radius: 8px; border: none; background: transparent; color: $text-color-accent; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; } &:hover:not(.active) { color: $text-color; } }
	.picker-filters { display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; }
	.filter-btn { padding: 6px 12px; border-radius: 8px; border: 1px solid $bg-color-accent; background: transparent; color: $text-color-accent; font-size: 12px; cursor: pointer; transition: all 150ms ease; &.active { background: $accent-color-hover; color: $bg-color; border-color: $accent-color-hover; } }
	.tier-filter-btn { border-color: var(--tier-bg); &:hover, &.active { background: var(--tier-bg); color: var(--tier-text); border-color: var(--tier-bg); } }
	.error-msg { color: #ff6b6b; font-size: 14px; }
	.start-btn { display: flex; align-items: center; gap: 8px; padding: 12px 28px; border-radius: 12px; background: $accent-color-hover; color: $bg-color; fill: $bg-color; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 150ms ease, opacity 150ms ease; &:hover { transform: scale(1.03); } &:disabled { opacity: 0.5; cursor: not-allowed; } }
	.game-header { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 11px; color: $text-color-accent; font-weight: 600; text-transform: uppercase; }
	.stat-value { font-size: 20px; font-weight: 700; }
	.clue-card { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 20px 24px; border-radius: 14px; background: $accent-color; border: 1px solid $bg-color-accent; width: 100%; }
	.clue-label { font-size: 12px; font-weight: 600; color: $text-color-accent; text-transform: uppercase; letter-spacing: 0.5px; }
	.episode-list { display: flex; flex-direction: column; gap: 8px; width: 100%; }
	.episode-title { font-size: 16px; font-weight: 600; color: $text-color; text-align: center; font-style: italic; padding: 6px 12px; border-radius: 8px; background: rgba(255,255,255,0.05); }
	.question-text { font-size: 15px; color: $text-color-accent; margin: 0; font-weight: 500; }
	.options-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; width: 100%; }
	.option-card { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px; border-radius: 12px; background: $accent-color; border: 2px solid transparent; cursor: pointer; transition: all 200ms ease;
		&:hover:not(:disabled) { border-color: $accent-color-hover; transform: scale(1.02); }
		&.correct { border-color: #51cf66; background: rgba(81,207,102,0.15); }
		&.wrong { border-color: #ff6b6b; background: rgba(255,107,107,0.15); }
		&.dimmed { opacity: 0.4; }
		&:disabled { cursor: default; }
	}
	.option-poster { width: 80px; height: 110px; object-fit: cover; border-radius: 8px; }
	.option-name { font-size: 13px; font-weight: 600; text-align: center; word-break: break-word; }
	.round-feedback { text-align: center; }
	.feedback-correct { font-size: 18px; font-weight: 700; color: #51cf66; }
	.feedback-wrong { font-size: 18px; font-weight: 700; color: #ff6b6b; }
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
