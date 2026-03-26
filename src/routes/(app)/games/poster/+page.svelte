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

	// --- Filter state ---
	let filterMode: FilterMode = $state("status");
	let enabledStatuses: WatchedStatus[] = $state(["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"]);
	let tiers: Tier[] = $state([]);
	let enabledTierIds: number[] = $state([]);
	let tiersLoaded = $state(false);
	let statusCounts: Record<string, number> = $state({});

	// --- Game state ---
	let allItems: Media[] = $state([]);
	let loading = $state(true);
	let error = $state("");
	let gamePhase: "setup" | "playing" | "result" = $state("setup");

	let currentItem: Media | null = $state(null);
	let options: string[] = $state([]);
	let correctIndex = $state(0);
	let blurLevel = $state(10); // starts blurry
	let guessesLeft = $state(1);
	let selectedAnswer: number | null = $state(null);
	let answered = $state(false);
	let wasCorrect = $state(false);
	let score = $state(0);
	let round = $state(0);
	let totalRounds = 10;
	let correctCount = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);
	let usedItems = new Set<string>();
	let posterReady = $state(false);

	const BLUR_LEVELS = [10, 7, 4, 2, 0]; // progressively clearer

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) {
			if (enabledStatuses.length <= 1) return;
			enabledStatuses = enabledStatuses.filter((x) => x !== s);
		} else { enabledStatuses = [...enabledStatuses, s]; }
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

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr];
		for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; }
		return a;
	}

	function setupRound() {
		const withPoster = allItems.filter((m) => getPoster(m) && !usedItems.has(m.name ?? ""));
		if (withPoster.length < 4) return false;
		const pool = shuffle(withPoster);
		currentItem = pool[0];
		usedItems.add(currentItem.name ?? "");

		// Pick 3 wrong answers
		const wrongNames = pool.slice(1, 4).map((m) => m.name ?? "Unknown");
		const allOpts = shuffle([currentItem.name ?? "Unknown", ...wrongNames]);
		options = allOpts;
		correctIndex = allOpts.indexOf(currentItem.name ?? "Unknown");

		blurLevel = BLUR_LEVELS[0];
		guessesLeft = 2;
		selectedAnswer = null;
		answered = false;
		wasCorrect = false;		posterReady = false;		return true;
	}

	async function startGame() {
		await loadItems();
		const withPoster = allItems.filter((m) => getPoster(m));
		if (withPoster.length < 4) { error = "You need at least 4 items with posters."; return; }
		error = "";
		score = 0; round = 0; correctCount = 0; streak = 0; bestStreak = 0;
		usedItems.clear();
		totalRounds = Math.min(10, Math.floor(withPoster.length / 2));
		if (!setupRound()) { error = "Couldn't set up the game."; return; }
		gamePhase = "playing";
	}

	function selectAnswer(index: number) {
		if (answered) return;
		selectedAnswer = index;

		answered = true;
		blurLevel = 0; // reveal fully

		if (index === correctIndex) {
			wasCorrect = true;
			score += 100;
			correctCount++;
			streak++;
			if (streak > bestStreak) bestStreak = streak;
		} else {
			wasCorrect = false;
			streak = 0;
		}

		setTimeout(() => {
			round++;
			if (round >= totalRounds || !setupRound()) {
				gamePhase = "result";
			}
		}, 2000);
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "result") {
			axios.post("/gamescore", { game: "poster", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head>
	<title>Guess the Poster</title>
</svelte:head>

<div class="poster-page">
	<PageTitle title="Guess the Poster" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Can you recognize your watchlist from a blurry poster?</p>

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
			{#if loading}<SpinnerTiny /> Loading...
			{:else}<Icon i="eye-closed" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}/{totalRounds}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		{#if currentItem}
			<div class="poster-card">
				<div class="poster-frame">
					<img
						class="blurred-poster"
						src={getPoster(currentItem)}
						alt="?"
						style="filter: blur({blurLevel}px); opacity: {posterReady ? 1 : 0}; transition: {posterReady && answered ? 'filter 500ms ease' : 'none'};"
						onload={() => posterReady = true}
					/>
				</div>

				{#if answered}
					<div class="answer-reveal" class:correct={wasCorrect} class:wrong={!wasCorrect}>
						{wasCorrect ? "✅" : "❌"} {currentItem.name}
					</div>
				{/if}
			</div>

			<div class="options-grid">
				{#each options as opt, i}
					{@const letter = ["A", "B", "C", "D"][i]}
					<button
						class="plain option-btn"
						class:correct={answered && i === correctIndex}
						class:wrong={answered && selectedAnswer === i && i !== correctIndex}
						class:wrong-guess={!answered && selectedAnswer === null && false}
						disabled={answered}
						onclick={() => selectAnswer(i)}
					>
						<span class="option-letter">{letter}</span>
						<span class="option-text">{opt}</span>
					</button>
				{/each}
			</div>
		{/if}

	{:else if gamePhase === "result"}
		{@const pct = totalRounds > 0 ? Math.round((correctCount / totalRounds) * 100) : 0}
		<div class="result-card">
			<div class="result-emoji">{pct >= 90 ? "🏆" : pct >= 70 ? "🌟" : pct >= 50 ? "👍" : pct >= 30 ? "🤔" : "💀"}</div>
			<h2>Game Complete!</h2>
			<div class="result-stats">
				<div class="result-stat"><span class="result-stat-value">{score.toLocaleString()}</span><span class="result-stat-label">Total Score</span></div>
				<div class="result-stat"><span class="result-stat-value">{correctCount}/{totalRounds}</span><span class="result-stat-label">Correct</span></div>
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
	.poster-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		padding: 20px;
		width: 100%;
		max-width: 700px;
		margin: 0 auto;
	}

	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; }

	.filter-mode-toggle { display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px; }

	.filter-mode-btn {
		padding: 6px 14px; border-radius: 8px; border: none; background: transparent;
		color: $text-color-accent; font-size: 12px; font-weight: 600; cursor: pointer;
		transition: all 150ms ease;
		&.active { background: $accent-color-hover; color: $bg-color; }
		&:hover:not(.active) { color: $text-color; }
	}

	.picker-filters { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: center; }

	.filter-btn {
		padding: 8px 14px; border-radius: 8px; border: 2px solid $text-color;
		background: $bg-color; color: $text-color; font-size: 13px; font-weight: 600;
		cursor: pointer; transition: all 150ms ease;
		&:hover, &.active { background: $accent-color-hover; color: $bg-color; }
		&.active { border-color: $bg-color; }
	}

	.tier-filter-btn {
		border-color: var(--tier-bg);
		&:hover, &.active { background: var(--tier-bg); color: var(--tier-text); border-color: var(--tier-bg); }
	}

	.no-tiers { font-size: 13px; color: $text-color-accent; }
	.error-msg { color: $error; font-size: 14px; }

	.start-btn {
		display: flex; align-items: center; justify-content: center; gap: 10px;
		padding: 14px 36px; border-radius: 10px; border: 2px solid $text-color;
		background: $bg-color; color: $text-color; fill: $text-color;
		font-size: 18px; font-weight: 700; cursor: pointer;
		transition: all 150ms ease;
		&:hover:not(:disabled) { background: $text-color; color: $bg-color; fill: $bg-color; }
		&:disabled { opacity: 0.5; cursor: not-allowed; }
	}

	.game-header { display: flex; gap: 16px; align-items: center; flex-wrap: wrap; justify-content: center; width: 100%; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 10px; text-transform: uppercase; color: $text-color-accent; font-weight: 600; }
	.stat-value { font-size: 18px; font-weight: 700; color: $text-color; }

	.poster-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		width: 100%;
	}

	.poster-frame {
		width: 220px;
		height: 320px;
		border-radius: 16px;
		overflow: hidden;
		border: 3px solid $bg-color-accent;
		background: $accent-color;
	}

	.blurred-poster {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.answer-reveal {
		font-size: 20px;
		font-weight: 700;
		padding: 10px 20px;
		border-radius: 12px;
		animation: reveal-pop 400ms ease;

		&.correct { color: #2ecc71; background: rgba(46,204,113,0.15); }
		&.wrong { color: #e74c3c; background: rgba(231,76,60,0.15); }
	}

	@keyframes reveal-pop {
		0% { transform: scale(0.8); opacity: 0; }
		100% { transform: scale(1); opacity: 1; }
	}

	.options-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 10px;
		width: 100%;
	}

	.option-btn {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 14px 16px;
		border-radius: 12px;
		border: 2px solid $bg-color-accent;
		background: $accent-color;
		color: $text-color;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		transition: all 200ms ease;
		text-align: left;

		&:hover:not(:disabled) { border-color: $accent-color-hover; background: $bg-color-accent; }
		&.correct { border-color: #2ecc71; background: rgba(46,204,113,0.2); color: #2ecc71; }
		&.wrong { border-color: #e74c3c; background: rgba(231,76,60,0.2); color: #e74c3c; }
		&:disabled:not(.correct):not(.wrong) { opacity: 0.7; cursor: default; }
	}

	.option-letter {
		display: flex; align-items: center; justify-content: center;
		width: 28px; height: 28px; border-radius: 8px;
		background: $bg-color-accent; font-size: 13px; font-weight: 700; flex-shrink: 0;
	}

	.option-text { flex: 1; }

	// --- Result ---
	.result-card {
		display: flex; flex-direction: column; align-items: center; gap: 20px;
		padding: 32px; border-radius: 20px; background: $accent-color;
		border: 1px solid $bg-color-accent; width: 100%;
		h2 { margin: 0; font-size: 24px; font-weight: 700; }
	}

	.result-emoji { font-size: 64px; animation: result-bounce 500ms ease; }

	@keyframes result-bounce {
		0% { transform: scale(0); }
		50% { transform: scale(1.3); }
		100% { transform: scale(1); }
	}

	.result-stats { display: flex; gap: 24px; flex-wrap: wrap; justify-content: center; }
	.result-stat { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.result-stat-value { font-size: 24px; font-weight: 700; color: $text-color; }
	.result-stat-label { font-size: 12px; color: $text-color-accent; }
	.result-actions { display: flex; flex-direction: column; align-items: center; gap: 10px; width: 100%; }
	.back-link { font-size: 14px; color: $text-color-accent; text-decoration: none; &:hover { color: $text-color; } }
</style>
