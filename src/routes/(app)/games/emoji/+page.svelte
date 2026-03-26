<script lang="ts">
	import { onMount, onDestroy } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Icon from "@/lib/Icon.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type FilterMode = "status" | "tier";

	const ROUND_TIME = 10; // seconds per round
	const REVEAL_INTERVAL = 400; // ms between word reveals

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

	let words: string[] = $state([]);
	let revealedCount = $state(0); // how many revealable (even-index) words are shown
	let timeLeft = $state(ROUND_TIME);
	let timerRef: ReturnType<typeof setInterval> | null = null;
	let revealRef: ReturnType<typeof setInterval> | null = null;

	let options: Media[] = $state([]);
	let correctIndex = $state(0);
	let selectedAnswer: number | null = $state(null);
	let answered = $state(false);
	let wasCorrect = $state(false);
	let score = $state(0);
	let round = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let usedItems = new Set<string>();

	// Derived: which words are visible? Even-indexed words up to revealedCount are revealed.
	let revealableIndices = $derived(words.map((_, i) => i).filter((i) => i % 2 === 0));
	let visibleSet = $derived(new Set(revealableIndices.slice(0, revealedCount)));

	function clearTimers() {
		if (timerRef) { clearInterval(timerRef); timerRef = null; }
		if (revealRef) { clearInterval(revealRef); revealRef = null; }
	}

	function startTimers() {
		clearTimers();
		timeLeft = ROUND_TIME;
		revealedCount = 0;
		timerRef = setInterval(() => {
			timeLeft = Math.max(0, timeLeft - 0.1);
			if (timeLeft <= 0) { onTimeUp(); }
		}, 100);
		revealRef = setInterval(() => {
			if (revealedCount < revealableIndices.length) revealedCount++;
		}, REVEAL_INTERVAL);
	}

	function onTimeUp() {
		if (answered) return;
		clearTimers();
		selectedAnswer = null;
		answered = true;
		wasCorrect = false;
		streak = 0;
		revealedCount = revealableIndices.length + words.length; // reveal all
		setTimeout(() => { gamePhase = "gameover"; }, 5000);
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

	async function fetchDetails(m: Media): Promise<Media | null> {
		try {
			const ct = getContentType(m);
			const id = ct === "game" ? m.ids?.igdb : m.ids?.tmdb;
			if (!id) return null;
			const r = ct === "game" ? await axios.get(`/game/${id}`) : await axios.get(`/content/${ct}/${id}`);
			return r.data;
		} catch { return null; }
	}

	async function setupRound(): Promise<boolean> {
		clearTimers();
		let pool = allItems.filter((m) => !usedItems.has(m.name ?? ""));
		if (pool.length < 4) { usedItems.clear(); pool = [...allItems]; }
		if (pool.length < 4) return false;

		const candidates = shuffle(pool);
		for (const candidate of candidates) {
			const detail = await fetchDetails(candidate);
			if (!detail?.summary || detail.summary.length < 20) continue;

			usedItems.add(candidate.name ?? "");
			const wrongItems = shuffle(pool.filter((m) => m.name !== candidate.name)).slice(0, 3);
			if (wrongItems.length < 3) continue;

			const allOpts = shuffle([candidate, ...wrongItems]);
			options = allOpts;
			correctIndex = allOpts.indexOf(candidate);
			let overview = detail.summary;
			const name = candidate.name ?? "";
			overview = overview.replace(new RegExp(name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'), '???');
			words = overview.split(/\s+/).filter(Boolean);
			revealedCount = 0;
			selectedAnswer = null;
			answered = false;
			wasCorrect = false;
			startTimers();
			return true;
		}
		return false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) { error = "You need at least 4 items."; return; }
		error = ""; score = 0; round = 0; streak = 0; bestStreak = 0;
		usedItems.clear();
		if (!(await setupRound())) { error = "Couldn't find items with descriptions."; return; }
		gamePhase = "playing";
	}

	async function selectAnswer(index: number) {
		if (answered) return;
		clearTimers();
		selectedAnswer = index;
		answered = true;
		revealedCount = revealableIndices.length + words.length; // reveal all on answer

		if (index === correctIndex) {
			wasCorrect = true;
			streak++;
			const timeBonus = Math.round(timeLeft * 10); // up to 100 bonus for speed
			score += 100 + streak * 25 + timeBonus;
			if (streak > bestStreak) bestStreak = streak;
			setTimeout(async () => {
				round++;
				if (!(await setupRound())) gamePhase = "gameover";
			}, 3000);
		} else {
			wasCorrect = false;
			streak = 0;
			setTimeout(() => { gamePhase = "gameover"; }, 5000);
		}
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w92${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) return `https://images.igdb.com/igdb/image/upload/t_thumb/${m.extPosterPath}.jpg`;
		if (m.type === MediaTypeE.malManga) return m.extPosterPath;
		return "";
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });
	onDestroy(() => { clearTimers(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			clearTimers();
			axios.post("/gamescore", { game: "emoji", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Plot Twist</title></svelte:head>

<div class="emoji-page">
	<PageTitle title="Plot Twist" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Can you match the description to the right item?</p>

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
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="document" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		<div class="timer-bar-wrap">
			<div class="timer-bar" style="width: {(timeLeft / ROUND_TIME) * 100}%;" class:timer-low={timeLeft <= 3}></div>
			<span class="timer-label">{timeLeft.toFixed(1)}s</span>
		</div>

		<div class="overview-card">
			<p class="overview-text">
				{#each words as word, i}
					{#if answered || visibleSet.has(i)}
						<span class="word revealed">{word}</span>
					{:else}
						<span class="word blurred">{word}</span>
					{/if}
					{' '}
				{/each}
			</p>
		</div>

		<div class="options-grid">
			{#each options as opt, i}
				{@const poster = getPoster(opt)}
				<button
					class="plain option-card"
					class:correct={answered && i === correctIndex}
					class:wrong={answered && selectedAnswer === i && i !== correctIndex}
					disabled={answered}
					onclick={() => selectAnswer(i)}
				>
					{#if poster}<img src={poster} alt="" class="option-poster" />{/if}
					<span class="option-name">{opt.name}</span>
				</button>
			{/each}
		</div>

		{#if answered}
			<div class="round-feedback">
				{#if wasCorrect}
					<span class="feedback-correct">✅ Correct! +{100 + streak * 25 + Math.round(timeLeft * 10)} (⏱ +{Math.round(timeLeft * 10)} speed bonus)</span>
				{:else if selectedAnswer !== null}
					<span class="feedback-wrong">❌ Wrong! It was {options[correctIndex]?.name}</span>
				{:else}
					<span class="feedback-wrong">⏰ Time's up! It was {options[correctIndex]?.name}</span>
				{/if}
			</div>
		{/if}

	{:else if gamePhase === "gameover"}
		<div class="result-card">
			<div class="result-emoji">{bestStreak >= 10 ? "🏆" : bestStreak >= 5 ? "🌟" : bestStreak >= 3 ? "👍" : "💀"}</div>
			<h2>Game Over!</h2>
			<div class="result-stats">
				<div class="result-stat"><span class="result-stat-value">{score.toLocaleString()}</span><span class="result-stat-label">Total Score</span></div>
				<div class="result-stat"><span class="result-stat-value">{round} Rounds</span><span class="result-stat-label">Survived</span></div>
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
	.emoji-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 700px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; }
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
	.overview-card { background: $accent-color; border-radius: 14px; padding: 20px 24px; border: 1px solid $bg-color-accent; width: 100%; }
	.overview-text { font-size: 15px; line-height: 1.8; color: $text-color; margin: 0; font-style: italic; }
	.word { display: inline; transition: filter 300ms ease, opacity 300ms ease; }
	.word.revealed { filter: none; opacity: 1; }
	.word.blurred { filter: blur(5px); opacity: 0.5; user-select: none; pointer-events: none; }
	.timer-bar-wrap { width: 100%; height: 28px; background: $accent-color; border-radius: 14px; position: relative; overflow: hidden; border: 1px solid $bg-color-accent; }
	.timer-bar { height: 100%; background: $accent-color-hover; border-radius: 14px; transition: width 100ms linear; }
	.timer-bar.timer-low { background: #ff6b6b; }
	.timer-label { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); font-size: 12px; font-weight: 700; color: $text-color; }
	.round-feedback { text-align: center; }
	.feedback-correct { font-size: 16px; font-weight: 700; color: #51cf66; }
	.feedback-wrong { font-size: 16px; font-weight: 700; color: #ff6b6b; }
	.options-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; width: 100%; }
	.option-card { display: flex; align-items: center; gap: 10px; padding: 12px; border-radius: 12px; background: $accent-color; border: 2px solid $bg-color-accent; cursor: pointer; transition: all 200ms ease; text-align: left; &:hover:not(:disabled) { border-color: $accent-color-hover; transform: translateY(-2px); } &.correct { border-color: #51cf66; background: rgba(81,207,102,0.15); } &.wrong { border-color: #ff6b6b; background: rgba(255,107,107,0.15); } &:disabled { cursor: default; } }
	.option-poster { width: 40px; height: 56px; object-fit: cover; border-radius: 6px; flex-shrink: 0; }
	.option-name { font-size: 13px; font-weight: 600; color: $text-color; }
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
