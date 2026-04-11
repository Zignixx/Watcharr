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

	const ROUND_TIME = 60;

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

	/** actorId → [{showName, character}] across all fetched shows */
	type ActorRoleMap = Map<number, { actorName: string; photo: string; roles: { showName: string; character: string }[] }>;

	type QuestionMode = "who_else" | "odd_character" | "match_character";

	interface RoundData {
		mode: QuestionMode;
		/** The character shown as the prompt */
		promptCharacter: string;
		/** The show that character is from */
		promptShow: string;
		/** Prompt character image (Jikan) */
		promptCharImage: string | null;
		/** For display after answer */
		actorName: string;
		/** Question text */
		question: string;
		options: string[];
		/** Character image URLs per option (from Jikan) */
		optionImages: (string | null)[];
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
	let loadingMsg = $state("");
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

	let creditsCache: Map<string, ShowCredits> = new Map();
	let actorRoles: ActorRoleMap = new Map();
	let usedQuestionKeys = new Set<string>();

	// --- Jikan character image cache & rate limiter ---
	let charImageCache: Map<string, string | null> = new Map();
	let jikanLastCall = 0;
	const JIKAN_MIN_INTERVAL = 500; // 2 req/s (safe under Jikan's 3/s limit)
	const JIKAN_MAX_RETRIES = 3;

	/** Clean character name: strip parenthesized suffixes like "(voice)" and trim */
	function cleanCharName(raw: string): string {
		return raw.replace(/\s*\([^)]*\)\s*/g, "").trim();
	}

	/** Rate-limited wait before next Jikan request */
	async function jikanRateWait(): Promise<void> {
		const now = Date.now();
		const waitMs = Math.max(0, JIKAN_MIN_INTERVAL - (now - jikanLastCall));
		if (waitMs > 0) await new Promise((r) => setTimeout(r, waitMs));
		jikanLastCall = Date.now();
	}

	/** Rate-limited fetch of character image from Jikan API v4 with retry on 429 */
	async function fetchCharImage(rawName: string): Promise<string | null> {
		const name = cleanCharName(rawName);
		if (!name) return null;
		if (charImageCache.has(name)) return charImageCache.get(name)!;

		for (let attempt = 0; attempt < JIKAN_MAX_RETRIES; attempt++) {
			await jikanRateWait();
			try {
				const resp = await fetch(`https://api.jikan.moe/v4/characters?q=${encodeURIComponent(name)}`, {
					signal: AbortSignal.timeout(8000),
				});
				if (resp.status === 429) {
					// Back off exponentially: 2s, 4s, 8s
					const backoff = 2000 * Math.pow(2, attempt);
					jikanLastCall = Date.now() + backoff;
					await new Promise((r) => setTimeout(r, backoff));
					continue;
				}
				if (!resp.ok) { charImageCache.set(name, null); return null; }
				const json = await resp.json();
				const data: any[] = json?.data ?? [];
				const lowerName = name.toLowerCase();
				const match = data.find((d: any) => (d.name ?? "").toLowerCase() === lowerName);
				const url = match?.images?.jpg?.image_url ?? match?.images?.webp?.image_url ?? null;
				charImageCache.set(name, url);
				return url;
			} catch {
				if (attempt === JIKAN_MAX_RETRIES - 1) {
					charImageCache.set(name, null);
					return null;
				}
				await new Promise((r) => setTimeout(r, 1000));
			}
		}
		charImageCache.set(name, null);
		return null;
	}

	/** Fetch images for an array of character names (sequential, rate-limit aware) */
	async function fetchCharImages(names: string[], onProgress?: (done: number, total: number) => void): Promise<(string | null)[]> {
		const results: (string | null)[] = [];
		for (let i = 0; i < names.length; i++) {
			results.push(await fetchCharImage(names[i]));
			onProgress?.(i + 1, names.length);
		}
		return results;
	}

	/** Pre-fetch Jikan images for all characters likely to appear in questions */
	async function prefetchCharacterImages() {
		// Collect unique character names from actors with ≥2 roles (question-relevant)
		const charNames = new Set<string>();
		for (const [, actor] of actorRoles) {
			if (actor.roles.length >= 2) {
				for (const r of actor.roles) charNames.add(r.character);
			}
		}
		// Filter out already cached
		const toFetch = [...charNames].filter((n) => !charImageCache.has(cleanCharName(n)));
		if (toFetch.length === 0) return;

		await fetchCharImages(toFetch, (done, total) => {
			loadingMsg = `Loading character images (${done}/${total})...`;
		});
	}

	/** Extract raw character name from option string like "Eren Yeager (Attack on Titan)" */
	function extractCharName(opt: string): string {
		const match = opt.match(/^(.+?)\s*\(/);
		return match ? match[1].trim() : opt.trim();
	}

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

	/** Build a map: actorId → all roles across all fetched shows */
	function buildActorRoleMap() {
		actorRoles.clear();
		for (const [, sc] of creditsCache) {
			for (const c of sc.cast) {
				if (!actorRoles.has(c.id)) {
					actorRoles.set(c.id, { actorName: c.name, photo: c.profile_path ?? "", roles: [] });
				}
				const entry = actorRoles.get(c.id)!;
				if (!entry.roles.some((r) => r.showName === sc.item.name && r.character === c.character)) {
					entry.roles.push({ showName: sc.item.name!, character: c.character });
				}
				if (!entry.photo && c.profile_path) entry.photo = c.profile_path;
			}
		}
	}

	/** Collect all unique characters from all fetched credits (for wrong options) */
	function getAllCharacters(): string[] {
		const chars = new Set<string>();
		for (const [, sc] of creditsCache) {
			for (const c of sc.cast) chars.add(c.character);
		}
		return [...chars];
	}

	// ========== QUESTION GENERATORS ==========

	/**
	 * Mode A — "who_else":
	 * "The voice actor of [Character] in [Show] also voices which other character?"
	 * Requires an actor that appears in ≥2 different shows in the user's watchlist.
	 * 4 options: 1 correct (another character from another show), 3 wrong characters.
	 */
	function genWhoElseQuestion(): RoundData | null {
		// Find actors with roles in ≥2 shows
		const multiShowActors = [...actorRoles.entries()].filter(([, a]) => {
			const shows = new Set(a.roles.map((r) => r.showName));
			return shows.size >= 2 && a.photo;
		});
		if (multiShowActors.length === 0) return null;

		for (const [actorId, actor] of shuffle(multiShowActors)) {
			const qKey = `whoelse_${actorId}`;
			if (usedQuestionKeys.has(qKey)) continue;

			const roles = shuffle([...actor.roles]);
			const promptRole = roles[0];
			const answerRole = roles.find((r) => r.showName !== promptRole.showName);
			if (!answerRole) continue;

			// Wrong options: characters NOT voiced by this actor
			const allChars = getAllCharacters();
			const actorCharNames = new Set(actor.roles.map((r) => r.character));
			const wrongPool = allChars.filter((c) => !actorCharNames.has(c) && c !== answerRole.character);
			if (wrongPool.length < 3) continue;
			const wrongChars = shuffle(wrongPool).slice(0, 3);

			const opts = shuffle([`${answerRole.character} (${answerRole.showName})`, ...wrongChars.map((c) => {
				// Find which show this wrong character is from for display
				for (const [, sc] of creditsCache) {
					const found = sc.cast.find((cm) => cm.character === c);
					if (found) return `${c} (${sc.item.name})`;
				}
				return c;
			})]);

			const correctOpt = `${answerRole.character} (${answerRole.showName})`;
			const correctIdx = opts.indexOf(correctOpt);

			usedQuestionKeys.add(qKey);
			return {
				mode: "who_else",
				promptCharacter: promptRole.character,
				promptShow: promptRole.showName,
				promptCharImage: null,
				actorName: actor.actorName,
				question: `The voice actor of "${promptRole.character}" (${promptRole.showName}) also voices which of these characters?`,
				options: opts,
				optionImages: [],
				correctIndex: correctIdx,
				explanation: `${actor.actorName} voices both "${promptRole.character}" in ${promptRole.showName} and "${answerRole.character}" in ${answerRole.showName}.`,
			};
		}
		return null;
	}

	/**
	 * Mode B — "odd_character":
	 * "3 of these characters share the same voice actor. Which one does NOT?"
	 * Requires an actor with ≥3 roles.
	 */
	function genOddCharacterQuestion(): RoundData | null {
		const multiRoleActors = [...actorRoles.entries()].filter(([, a]) => a.roles.length >= 3 && a.photo);
		if (multiRoleActors.length === 0) return null;

		for (const [actorId, actor] of shuffle(multiRoleActors)) {
			const qKey = `odd_${actorId}`;
			if (usedQuestionKeys.has(qKey)) continue;

			const sameRoles = shuffle([...actor.roles]).slice(0, 3);
			// Find a character NOT voiced by this actor
			const actorCharNames = new Set(actor.roles.map((r) => r.character));
			const allChars = getAllCharacters();
			const oddPool = allChars.filter((c) => !actorCharNames.has(c));
			if (oddPool.length === 0) continue;
			const oddChar = oddPool[Math.floor(Math.random() * oddPool.length)];

			// Find show name for odd character
			let oddShowName = "";
			for (const [, sc] of creditsCache) {
				const found = sc.cast.find((cm) => cm.character === oddChar);
				if (found) { oddShowName = sc.item.name!; break; }
			}

			const correctOpt = `${oddChar} (${oddShowName})`;
			const opts = shuffle([...sameRoles.map((r) => `${r.character} (${r.showName})`), correctOpt]);
			const correctIdx = opts.indexOf(correctOpt);

			usedQuestionKeys.add(qKey);
			return {
				mode: "odd_character",
				promptCharacter: sameRoles[0].character,
				promptShow: sameRoles[0].showName,
				promptCharImage: null,
				actorName: actor.actorName,
				question: `3 of these characters share the same voice actor. Which one does NOT?`,
				options: opts,
				optionImages: [],
				correctIndex: correctIdx,
				explanation: `${actor.actorName} voices ${sameRoles.map((r) => `"${r.character}" (${r.showName})`).join(", ")} — but NOT "${oddChar}".`,
			};
		}
		return null;
	}

	/**
	 * Mode C — "match_character":
	 * "In [Show], which character is voiced by the same actor as [Character] from [OtherShow]?"
	 * Show 4 characters from the target show, pick the one with the same voice actor.
	 */
	function genMatchCharacterQuestion(): RoundData | null {
		const multiShowActors = [...actorRoles.entries()].filter(([, a]) => {
			const shows = new Set(a.roles.map((r) => r.showName));
			return shows.size >= 2 && a.photo;
		});
		if (multiShowActors.length === 0) return null;

		for (const [actorId, actor] of shuffle(multiShowActors)) {
			const qKey = `match_${actorId}`;
			if (usedQuestionKeys.has(qKey)) continue;

			const roles = [...actor.roles];
			const showGroups = new Map<string, string[]>();
			for (const r of roles) {
				if (!showGroups.has(r.showName)) showGroups.set(r.showName, []);
				showGroups.get(r.showName)!.push(r.character);
			}

			// Need a prompt show and a target show
			const showNames = shuffle([...showGroups.keys()]);
			if (showNames.length < 2) continue;
			const promptShow = showNames[0];
			const targetShow = showNames[1];
			const promptChar = showGroups.get(promptShow)![0];
			const targetChar = showGroups.get(targetShow)![0];

			// Get other characters from target show as wrong options
			const targetCreditsEntry = [...creditsCache.values()].find((sc) => sc.item.name === targetShow);
			if (!targetCreditsEntry || targetCreditsEntry.cast.length < 4) continue;

			const wrongChars = targetCreditsEntry.cast
				.filter((c) => c.character !== targetChar && c.id !== actorId)
				.map((c) => c.character);
			if (wrongChars.length < 3) continue;

			const opts = shuffle([targetChar, ...shuffle(wrongChars).slice(0, 3)]);
			const correctIdx = opts.indexOf(targetChar);

			usedQuestionKeys.add(qKey);
			return {
				mode: "match_character",
				promptCharacter: promptChar,
				promptShow: promptShow,
				promptCharImage: null,
				actorName: actor.actorName,
				question: `In "${targetShow}", which character is voiced by the same actor as "${promptChar}" (${promptShow})?`,
				options: opts,
				optionImages: [],
				correctIndex: correctIdx,
				explanation: `${actor.actorName} voices "${promptChar}" in ${promptShow} and "${targetChar}" in ${targetShow}.`,
			};
		}
		return null;
	}

	async function setupRound(): Promise<boolean> {
		clearTimers();

		// Randomly pick a question mode
		const generators = shuffle([genWhoElseQuestion, genOddCharacterQuestion, genMatchCharacterQuestion]);
		let rd: RoundData | null = null;
		for (const gen of generators) {
			rd = gen();
			if (rd) break;
		}

		if (!rd) {
			usedQuestionKeys.clear();
			for (const gen of generators) {
				rd = gen();
				if (rd) break;
			}
		}

		if (!rd) return false;

		// Fetch Jikan character image for prompt character + all options
		rd.promptCharImage = await fetchCharImage(rd.promptCharacter);
		const charNames = rd.options.map(extractCharName);
		rd.optionImages = await fetchCharImages(charNames);

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
		actorRoles.clear();
		usedQuestionKeys.clear();
		loading = true;
		loadingMsg = "Loading watchlist...";

		// Pre-fetch credits for a batch of shows
		loadingMsg = `Loading cast data (0/${Math.min(allItems.length, 15)})...`;
		const batch = shuffle([...allItems]).slice(0, 15);
		let fetched = 0;
		await Promise.all(batch.map(async (item) => {
			await fetchCredits(item);
			fetched++;
			loadingMsg = `Loading cast data (${fetched}/${batch.length})...`;
		}));
		loadingMsg = "Building questions...";
		buildActorRoleMap();

		// Pre-fetch character images for all question-relevant characters
		loadingMsg = "Loading character images...";
		await prefetchCharacterImages();

		loading = false;
		loadingMsg = "";
		if (!(await setupRound())) {
			error = "Couldn't find enough shared voice actors. Try adding more shows to your watchlist!";
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
				// Every 5 rounds, fetch more credits to expand the pool
				if (round % 5 === 0) {
					const unfetched = allItems.filter((m) => !creditsCache.has(`${getContentType(m)}_${m.ids?.tmdb}`));
					if (unfetched.length > 0) {
						const extra = shuffle(unfetched).slice(0, 5);
						await Promise.all(extra.map((item) => fetchCredits(item)));
						buildActorRoleMap();
					}
				}
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
		<p class="subtitle">Do you know which characters share the same voice actor?</p>

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
		{#if loading && loadingMsg}
			<div class="loading-overlay">
				<SpinnerTiny />
				<span class="loading-text">{loadingMsg}</span>
			</div>
		{/if}
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

		<div class="character-card">
			{#if roundData.promptCharImage}
				<img src={roundData.promptCharImage} alt={roundData.promptCharacter} class="actor-photo" />
			{/if}
			<div class="character-info">
				<p class="question-text">{roundData.question}</p>
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
					{#if roundData.optionImages?.[i]}
						<img src={roundData.optionImages[i]} alt="" class="option-char-img" />
					{/if}
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
				<p class="actor-reveal">🎙️ {roundData.actorName}</p>
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
	.loading-overlay {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		padding: 24px 32px;
		border-radius: 14px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		animation: fade-in 200ms ease;
	}
	.loading-text { font-size: 14px; color: $text-color-accent; font-weight: 600; }
	@keyframes fade-in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
	.start-btn { display: flex; align-items: center; gap: 8px; padding: 12px 28px; border-radius: 12px; background: $accent-color-hover; color: $bg-color; fill: $bg-color; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 150ms ease, opacity 150ms ease; &:hover { transform: scale(1.03); } &:disabled { opacity: 0.5; cursor: not-allowed; } }

	.game-header { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 11px; color: $text-color-accent; font-weight: 600; text-transform: uppercase; }
	.stat-value { font-size: 20px; font-weight: 700; }

	.timer-bar-wrap { width: 100%; height: 28px; background: $accent-color; border-radius: 14px; position: relative; overflow: hidden; border: 1px solid $bg-color-accent; }
	.timer-bar { height: 100%; background: $accent-color-hover; border-radius: 14px; transition: width 100ms linear; }
	.timer-bar.timer-low { background: #ff6b6b; }
	.timer-label { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); font-size: 12px; font-weight: 700; color: $text-color; }

	.character-card {
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
		width: 90px;
		height: 120px;
		object-fit: cover;
		border-radius: 12px;
		flex-shrink: 0;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
	}

	.character-info {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.question-text {
		margin: 0;
		font-size: 15px;
		color: $text-color;
		line-height: 1.6;
		font-weight: 500;
	}

	.options-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; width: 100%; }
	.option-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 12px 10px;
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
	.option-char-img {
		width: 48px;
		height: 48px;
		object-fit: cover;
		border-radius: 50%;
		border: 2px solid $bg-color-accent;
		flex-shrink: 0;
	}
	.option-label { font-size: 13px; font-weight: 600; color: $text-color; line-height: 1.4; }

	.round-feedback { text-align: center; display: flex; flex-direction: column; gap: 6px; }
	.feedback-correct { font-size: 16px; font-weight: 700; color: #51cf66; }
	.feedback-wrong { font-size: 16px; font-weight: 700; color: #ff6b6b; }
	.explanation { font-size: 13px; color: $text-color-accent; margin: 0; line-height: 1.5; }
	.actor-reveal { font-size: 15px; font-weight: 700; color: $text-color; margin: 0; }

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
		.character-card { flex-direction: column; text-align: center; }
		.actor-photo { width: 70px; height: 94px; }
		.options-grid { grid-template-columns: 1fr; }
	}
</style>
