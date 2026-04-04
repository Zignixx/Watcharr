<script lang="ts">
	import { onMount, onDestroy } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier, TMDBContentCredits } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Icon from "@/lib/Icon.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type FilterMode = "status" | "tier";

	const ROUND_TIME = 15;

	// --- Actor & character data ---
	interface CastMember {
		id: number;
		name: string;
		character: string;
		profile_path: string;
		order: number;
	}

	interface ShowCredits {
		item: Media;
		cast: CastMember[];
	}

	type QuestionMode = "odd_show" | "match_character";

	interface RoundData {
		mode: QuestionMode;
		actorName: string;
		actorPhoto: string;
		/** Mode A: the show name the actor is known from (context hint) */
		contextShow?: string;
		/** Mode B: the specific show for character matching */
		targetShow?: string;
		options: string[];
		correctIndex: number;
		explanation: string;
	}

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
	let gamePhase: "setup" | "playing" | "gameover" = $state("setup");

	let roundData: RoundData | null = $state(null);
	let timeLeft = $state(ROUND_TIME);
	let timerRef: ReturnType<typeof setInterval> | null = null;

	let selectedAnswer: number | null = $state(null);
	let answered = $state(false);
	let wasCorrect = $state(false);
	let score = $state(0);
	let round = $state(0);
	let streak = $state(0);
	let bestStreak = $state(0);

	// Cache of credits per show
	let creditsCache: Map<string, ShowCredits> = new Map();
	let usedQuestionKeys = new Set<string>();

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

	function clearTimers() {
		if (timerRef) { clearInterval(timerRef); timerRef = null; }
	}

	function startTimer() {
		clearTimers();
		timeLeft = ROUND_TIME;
		timerRef = setInterval(() => {
			timeLeft = Math.max(0, timeLeft - 0.1);
			if (timeLeft <= 0) onTimeUp();
		}, 100);
	}

	function onTimeUp() {
		if (answered) return;
		clearTimers();
		selectedAnswer = null;
		answered = true;
		wasCorrect = false;
		streak = 0;
		scheduleAction(() => { gamePhase = "gameover"; }, 4000);
	}

	// --- Standard filter helpers ---
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
			// Only keep TV shows and movies (things that have TMDB credits with actors)
			allItems = allItems.filter((m) => m.type === MediaTypeE.tmdbShow || m.type === MediaTypeE.tmdbMovie);
		} catch { error = "Failed to load items."; allItems = []; }
		loading = false;
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr]; for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a;
	}

	function getContentType(m: Media): string {
		if (m.type === MediaTypeE.tmdbMovie) return "movie";
		if (m.type === MediaTypeE.tmdbShow) return "tv";
		return "movie";
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w200${m.extPosterPath}`;
		}
		return "";
	}

	async function fetchCredits(item: Media): Promise<ShowCredits | null> {
		const key = `${getContentType(item)}_${item.ids?.tmdb}`;
		if (creditsCache.has(key)) return creditsCache.get(key)!;
		try {
			const ct = getContentType(item);
			const id = item.ids?.tmdb;
			if (!id) return null;
			const r = await axios.get<TMDBContentCredits>(`/content/${ct}/${id}/credits`);
			const cast = (r.data?.cast ?? []).filter((c) => c.name && c.character && c.character !== "Self");
			if (cast.length < 2) return null;
			const sc: ShowCredits = { item, cast };
			creditsCache.set(key, sc);
			return sc;
		} catch { return null; }
	}

	// --- Question generators ---

	/**
	 * Mode A: "In which of these was [Actor] NOT a cast member?"
	 * Show actor photo + name. 4 shows: 3 they appeared in, 1 they didn't.
	 */
	async function genOddShowQuestion(): Promise<RoundData | null> {
		const pool = shuffle([...allItems]);

		for (const baseItem of pool) {
			const credits = await fetchCredits(baseItem);
			if (!credits || credits.cast.length < 3) continue;

			// Pick an actor with a profile image from this show
			const actorsWithPhoto = credits.cast.filter((c) => c.profile_path);
			if (actorsWithPhoto.length === 0) continue;
			const actor = actorsWithPhoto[Math.floor(Math.random() * actorsWithPhoto.length)];

			// Find other shows from watchlist where this actor also appears
			const appearsIn: Media[] = [baseItem];
			const otherShows = shuffle(pool.filter((m) => m.name !== baseItem.name));

			for (const other of otherShows) {
				if (appearsIn.length >= 3) break;
				const otherCredits = await fetchCredits(other);
				if (!otherCredits) continue;
				if (otherCredits.cast.some((c) => c.id === actor.id)) {
					appearsIn.push(other);
				}
			}

			if (appearsIn.length < 3) continue;

			// Find a show where the actor does NOT appear
			let oddItem: Media | null = null;
			for (const candidate of otherShows) {
				if (appearsIn.some((a) => a.name === candidate.name)) continue;
				const candCredits = await fetchCredits(candidate);
				if (!candCredits) continue;
				if (!candCredits.cast.some((c) => c.id === actor.id)) {
					oddItem = candidate;
					break;
				}
			}

			if (!oddItem) continue;

			const qKey = `odd_${actor.id}`;
			if (usedQuestionKeys.has(qKey)) continue;
			usedQuestionKeys.add(qKey);

			const showsIn = appearsIn.slice(0, 3);
			const allOptions = shuffle([...showsIn.map((s) => s.name!), oddItem.name!]);
			const correctIdx = allOptions.indexOf(oddItem.name!);

			return {
				mode: "odd_show",
				actorName: actor.name,
				actorPhoto: `https://image.tmdb.org/t/p/w185${actor.profile_path}`,
				options: allOptions,
				correctIndex: correctIdx,
				explanation: `${actor.name} was NOT in "${oddItem.name}". They appeared in: ${showsIn.map((s) => s.name).join(", ")}.`,
			};
		}
		return null;
	}

	/**
	 * Mode B: "[Actor] appeared in [Show]. Which character did they play?"
	 * Show actor photo + name + show name. 4 characters from that show, 1 correct.
	 */
	async function genCharacterQuestion(): Promise<RoundData | null> {
		const pool = shuffle([...allItems]);

		for (const item of pool) {
			const credits = await fetchCredits(item);
			if (!credits || credits.cast.length < 4) continue;

			// Pick an actor with a profile photo
			const actorsWithPhoto = credits.cast.filter((c) => c.profile_path);
			if (actorsWithPhoto.length === 0) continue;
			const actor = actorsWithPhoto[Math.floor(Math.random() * actorsWithPhoto.length)];

			const qKey = `char_${actor.id}_${item.ids?.tmdb}`;
			if (usedQuestionKeys.has(qKey)) continue;
			usedQuestionKeys.add(qKey);

			// Get 3 wrong characters from the same show
			const otherChars = credits.cast
				.filter((c) => c.id !== actor.id && c.character)
				.map((c) => c.character);
			if (otherChars.length < 3) continue;
			const wrongChars = shuffle(otherChars).slice(0, 3);

			const allOptions = shuffle([actor.character, ...wrongChars]);
			const correctIdx = allOptions.indexOf(actor.character);

			return {
				mode: "match_character",
				actorName: actor.name,
				actorPhoto: `https://image.tmdb.org/t/p/w185${actor.profile_path}`,
				targetShow: item.name!,
				options: allOptions,
				correctIndex: correctIdx,
				explanation: `${actor.name} played "${actor.character}" in "${item.name}".`,
			};
		}
		return null;
	}

	async function setupRound(): Promise<boolean> {
		clearTimers();

		// Alternate between question modes, with some randomness
		const mode: QuestionMode = Math.random() < 0.5 ? "odd_show" : "match_character";
		let rd: RoundData | null = null;

		if (mode === "odd_show") {
			rd = await genOddShowQuestion();
			if (!rd) rd = await genCharacterQuestion();
		} else {
			rd = await genCharacterQuestion();
			if (!rd) rd = await genOddShowQuestion();
		}

		if (!rd) {
			// Reset used keys and try once more
			usedQuestionKeys.clear();
			rd = await genCharacterQuestion();
			if (!rd) rd = await genOddShowQuestion();
		}

		if (!rd) return false;

		roundData = rd;
		selectedAnswer = null;
		answered = false;
		wasCorrect = false;
		startTimer();
		return true;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) {
			error = "You need at least 4 TV shows or movies in your selection to play.";
			return;
		}
		error = ""; score = 0; round = 0; streak = 0; bestStreak = 0;
		creditsCache.clear();
		usedQuestionKeys.clear();
		loading = true;

		// Pre-fetch credits for a batch to speed up first round
		const batch = shuffle([...allItems]).slice(0, 10);
		await Promise.all(batch.map((item) => fetchCredits(item)));

		loading = false;
		if (!(await setupRound())) {
			error = "Couldn't find enough cast data. Try adding more shows/movies to your watchlist!";
			return;
		}
		gamePhase = "playing";
	}

	function selectAnswer(index: number) {
		if (answered || !roundData) return;
		clearTimers();
		selectedAnswer = index;
		answered = true;

		if (index === roundData.correctIndex) {
			wasCorrect = true;
			streak++;
			const timeBonus = Math.round(timeLeft * 10);
			score += 100 + streak * 25 + timeBonus;
			if (streak > bestStreak) bestStreak = streak;
			scheduleAction(async () => {
				round++;
				if (!(await setupRound())) gamePhase = "gameover";
			}, 2500);
		} else {
			wasCorrect = false;
			streak = 0;
			scheduleAction(() => { gamePhase = "gameover"; }, 4000);
		}
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });
	onDestroy(() => { clearTimers(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			clearTimers();
			axios.post("/gamescore", { game: "voiceactor", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head><title>Voice Actor Quiz</title></svelte:head>

<div class="va-page" onclick={skipResult}>
	<PageTitle title="Voice Actor Quiz" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Test your knowledge of actors and their roles!</p>

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
			{#if loading}<SpinnerTiny /> Loading...{:else}<Icon i="person" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing" && roundData}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Round</span><span class="stat-value">{round + 1}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
		</div>

		<div class="timer-bar-wrap">
			<div class="timer-bar" style="width: {(timeLeft / ROUND_TIME) * 100}%;" class:timer-low={timeLeft <= 3}></div>
			<span class="timer-label">{timeLeft.toFixed(1)}s</span>
		</div>

		<div class="actor-card">
			{#if roundData.actorPhoto}
				<img src={roundData.actorPhoto} alt={roundData.actorName} class="actor-photo" />
			{/if}
			<div class="actor-info">
				<h2 class="actor-name">{roundData.actorName}</h2>
				{#if roundData.mode === "odd_show"}
					<p class="question-text">In which of these was this actor <strong>NOT</strong> a cast member?</p>
				{:else}
					<p class="question-text">Which character did they play in <strong>"{roundData.targetShow}"</strong>?</p>
				{/if}
			</div>
		</div>

		<div class="options-grid">
			{#each roundData.options as opt, i}
				<button
					class="plain option-card"
					class:correct={answered && i === roundData.correctIndex}
					class:wrong={answered && selectedAnswer === i && i !== roundData.correctIndex}
					disabled={answered}
					onclick={() => selectAnswer(i)}
				>
					<span class="option-label">{opt}</span>
				</button>
			{/each}
		</div>

		{#if answered}
			<div class="round-feedback">
				{#if wasCorrect}
					<span class="feedback-correct">✅ Correct! +{100 + streak * 25 + Math.round(timeLeft * 10)}</span>
				{:else if selectedAnswer !== null}
					<span class="feedback-wrong">❌ Wrong!</span>
				{:else}
					<span class="feedback-wrong">⏰ Time's up!</span>
				{/if}
				<p class="explanation">{roundData.explanation}</p>
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
	.va-page { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 20px; width: 100%; max-width: 700px; margin: 0 auto; }
	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; }
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

	.timer-bar-wrap { width: 100%; height: 28px; background: $accent-color; border-radius: 14px; position: relative; overflow: hidden; border: 1px solid $bg-color-accent; }
	.timer-bar { height: 100%; background: $accent-color-hover; border-radius: 14px; transition: width 100ms linear; }
	.timer-bar.timer-low { background: #ff6b6b; }
	.timer-label { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); font-size: 12px; font-weight: 700; color: $text-color; }

	.actor-card {
		display: flex;
		align-items: center;
		gap: 20px;
		padding: 20px 24px;
		border-radius: 16px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		width: 100%;
	}

	.actor-photo {
		width: 100px;
		height: 130px;
		object-fit: cover;
		border-radius: 12px;
		flex-shrink: 0;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
	}

	.actor-info {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.actor-name {
		margin: 0;
		font-size: 22px;
		font-weight: 700;
		color: $text-color;
	}

	.question-text {
		margin: 0;
		font-size: 15px;
		color: $text-color-accent;
		line-height: 1.5;
	}

	.options-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; width: 100%; }
	.option-card {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px 12px;
		border-radius: 12px;
		background: $accent-color;
		border: 2px solid $bg-color-accent;
		cursor: pointer;
		transition: all 200ms ease;
		text-align: center;
		min-height: 60px;
		&:hover:not(:disabled) { border-color: $accent-color-hover; transform: translateY(-2px); }
		&.correct { border-color: #51cf66; background: rgba(81,207,102,0.15); }
		&.wrong { border-color: #ff6b6b; background: rgba(255,107,107,0.15); }
		&:disabled { cursor: default; }
	}
	.option-label { font-size: 14px; font-weight: 600; color: $text-color; line-height: 1.4; }

	.round-feedback { text-align: center; display: flex; flex-direction: column; gap: 6px; }
	.feedback-correct { font-size: 16px; font-weight: 700; color: #51cf66; }
	.feedback-wrong { font-size: 16px; font-weight: 700; color: #ff6b6b; }
	.explanation { font-size: 13px; color: $text-color-accent; margin: 0; line-height: 1.5; }

	.result-card { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 32px; border-radius: 16px; background: $accent-color; border: 1px solid $bg-color-accent; }
	.result-emoji { font-size: 64px; animation: result-bounce 500ms ease; }
	@keyframes result-bounce { 0% { transform: scale(0); } 60% { transform: scale(1.3); } 100% { transform: scale(1); } }
	.result-stats { display: flex; gap: 24px; flex-wrap: wrap; justify-content: center; }
	.result-stat { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.result-stat-value { font-size: 24px; font-weight: 700; color: $text-color; }
	.result-stat-label { font-size: 12px; color: $text-color-accent; font-weight: 600; }
	.result-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
	.back-link { color: $text-color-accent; font-size: 14px; text-decoration: none; &:hover { color: $text-color; } }

	@media (max-width: 500px) {
		.actor-card { flex-direction: column; text-align: center; }
		.actor-photo { width: 80px; height: 104px; }
		.options-grid { grid-template-columns: 1fr; }
	}
</style>
