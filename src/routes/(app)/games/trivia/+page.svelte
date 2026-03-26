<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import type { Media, WatchedStatus, Tier } from "@/types";
	import { MediaTypeE } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import PageTitle from "@/lib/generic/PageTitle.svelte";

	type FilterMode = "status" | "tier";

	interface Question {
		text: string;
		image?: string;
		options: string[];
		correctIndex: number;
		category: string;
	}

	// --- Filter state (same as picker) ---
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
	let questions: Question[] = $state([]);
	let currentQ = $state(0);
	let score = $state(0);
	let selectedAnswer: number | null = $state(null);
	let answered = $state(false);
	let streak = $state(0);
	let bestStreak = $state(0);
	let showCorrect = $state(false);
	let lifelines = $state({ fiftyFifty: true, skip: true });
	let eliminatedOptions: number[] = $state([]);
	let totalQuestions = 15;
	let loadingQuestions = $state(false);

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

	// Difficulty tracking
	let difficulty = $derived(
		currentQ < 3 ? "easy" : currentQ < 6 ? "medium" : "hard"
	);

	let prizeLabels = ["100", "200", "300", "500", "1.000", "2.000", "4.000", "8.000", "16.000", "32.000", "64.000", "125.000", "250.000", "500.000", "1.000.000"];

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) {
			if (enabledStatuses.length <= 1) return;
			enabledStatuses = enabledStatuses.filter((x) => x !== s);
		} else {
			enabledStatuses = [...enabledStatuses, s];
		}
	}

	async function loadTiers() {
		if (tiersLoaded) return;
		try {
			const r = await axios.get("/tierlist");
			tiers = r.data || [];
			tiersLoaded = true;
			if (tiers.length > 0) enabledTierIds = tiers.map((t) => t.id);
		} catch { tiers = []; tiersLoaded = true; }
	}

	function toggleTier(id: number) {
		if (enabledTierIds.includes(id)) {
			if (enabledTierIds.length <= 1) return;
			enabledTierIds = enabledTierIds.filter((x) => x !== id);
		} else {
			enabledTierIds = [...enabledTierIds, id];
		}
	}

	function switchFilterMode(m: FilterMode) {
		if (filterMode === m) return;
		filterMode = m;
		if (m === "tier") loadTiers();
	}

	async function fetchStatusCounts() {
		const statuses: WatchedStatus[] = ["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"];
		const counts: Record<string, number> = {};
		await Promise.all(
			statuses.map(async (s) => {
				try {
					const r = await axios.get("/watched", { params: { status: s, limit: 1, page: 1 } });
					counts[s] = r.data?.totalResults ?? 0;
				} catch { counts[s] = 0; }
			}),
		);
		statusCounts = counts;
	}

	async function loadItems() {
		loading = true;
		error = "";
		try {
			if (filterMode === "tier") {
				await loadTiers();
				const watchedIds = new Set<number>();
				for (const tier of tiers) {
					if (enabledTierIds.includes(tier.id)) {
						for (const ti of tier.tierItems ?? []) watchedIds.add(ti.watchedId);
					}
				}
				if (watchedIds.size === 0) { allItems = []; loading = false; return; }
				const r = await axios.get("/watched", { params: { limit: 500, page: 1 } });
				const results: Media[] = r.data?.results ?? r.data ?? [];
				allItems = results.filter((m) => m.name && m.watched && watchedIds.has(m.watched.id));
			} else {
				const r = await axios.get("/watched", {
					params: { status: enabledStatuses.join(","), limit: 500, page: 1 },
				});
				const results = r.data?.results ?? r.data ?? [];
				allItems = results.filter((m: Media) => m.name);
			}
		} catch { error = "Failed to load items."; allItems = []; }
		loading = false;
	}

	function getPoster(m: Media): string {
		if (m.poster?.path) return `${baseURL}/${m.poster.path}`;
		if (!m.extPosterPath) return "";
		if (m.type === MediaTypeE.tmdbMovie || m.type === MediaTypeE.tmdbShow) {
			if (m.watched) return `${baseURL}/img${m.extPosterPath}`;
			return `https://image.tmdb.org/t/p/w200${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) return `https://images.igdb.com/igdb/image/upload/t_cover_big/${m.extPosterPath}.jpg`;
		if (m.type === MediaTypeE.malManga) return m.extPosterPath;
		return "";
	}

	// --- Fetch full details for a media item ---
	async function fetchDetails(m: Media): Promise<Media | null> {
		try {
			if (m.type === MediaTypeE.tmdbMovie) {
				const r = await axios.get(`/content/movie/${m.ids.tmdb}`);
				return r.data;
			} else if (m.type === MediaTypeE.tmdbShow) {
				const r = await axios.get(`/content/tv/${m.ids.tmdb}`);
				return r.data;
			} else if (m.type === MediaTypeE.igdbGame) {
				const r = await axios.get(`/game/${m.ids.igdb}`);
				return r.data;
			}
		} catch { /* skip */ }
		return null;
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr];
		for (let i = a.length - 1; i > 0; i--) {
			const j = Math.floor(Math.random() * (i + 1));
			[a[i], a[j]] = [a[j], a[i]];
		}
		return a;
	}

	function pickRandom<T>(arr: T[], n: number): T[] {
		return shuffle(arr).slice(0, n);
	}

	function randomInRange(min: number, max: number): number {
		return Math.floor(Math.random() * (max - min + 1)) + min;
	}

	// --- Question generators ---

	function genGenreQuestion(item: Media, detail: Media): Question | null {
		if (!detail.genres || detail.genres.length === 0) return null;
		const correctGenre = detail.genres[Math.floor(Math.random() * detail.genres.length)].name;
		const allGenres = [
			"Action", "Adventure", "Animation", "Comedy", "Crime", "Documentary",
			"Drama", "Family", "Fantasy", "History", "Horror", "Music", "Mystery",
			"Romance", "Science Fiction", "Thriller", "War", "Western",
			"Reality", "Talk Show", "Soap Opera",
		];
		const wrongGenres = allGenres.filter((g) => !detail.genres!.some((dg) => dg.name === g));
		const wrong = pickRandom(wrongGenres, 3);
		const opts = shuffle([correctGenre, ...wrong]);
		return {
			text: `Which genre does "${item.name}" belong to?`,
			image: getPoster(item),
			options: opts,
			correctIndex: opts.indexOf(correctGenre),
			category: "Genre",
		};
	}

	function genRuntimeQuestion(item: Media, detail: Media): Question | null {
		if (!detail.runtime || detail.runtime < 10) return null;
		const rt = detail.runtime;
		const options = new Set<number>([rt]);
		while (options.size < 4) {
			const offset = randomInRange(-30, 30);
			const val = rt + offset;
			if (val > 0 && val !== rt) options.add(val);
		}
		const sorted = [...options].sort((a, b) => a - b);
		return {
			text: `How long is "${item.name}" (in minutes)?`,
			image: getPoster(item),
			options: sorted.map((v) => `${v} min`),
			correctIndex: sorted.indexOf(rt),
			category: "Runtime",
		};
	}

	function genSeasonQuestion(item: Media, detail: Media): Question | null {
		if (!detail.seasons || detail.seasons.length < 2) return null;
		const count = detail.seasons.filter((s) => s.number > 0).length;
		if (count < 1) return null;
		const options = new Set<number>([count]);
		while (options.size < 4) {
			const offset = randomInRange(-3, 5);
			const val = count + offset;
			if (val > 0 && val !== count) options.add(val);
		}
		const sorted = [...options].sort((a, b) => a - b);
		return {
			text: `How many seasons does "${item.name}" have?`,
			image: getPoster(item),
			options: sorted.map(String),
			correctIndex: sorted.indexOf(count),
			category: "Seasons",
		};
	}

	function genEpisodeCountQuestion(item: Media, detail: Media): Question | null {
		if (!detail.seasons || detail.seasons.length === 0) return null;
		const total = detail.seasons.filter(s => s.number > 0).reduce((sum, s) => sum + s.episodeCount, 0);
		if (total < 2) return null;
		const options = new Set<number>([total]);
		while (options.size < 4) {
			const factor = randomInRange(50, 150) / 100;
			const val = Math.max(1, Math.round(total * factor));
			if (val !== total) options.add(val);
		}
		const sorted = [...options].sort((a, b) => a - b);
		return {
			text: `How many episodes does "${item.name}" have in total?`,
			image: getPoster(item),
			options: sorted.map(String),
			correctIndex: sorted.indexOf(total),
			category: "Episodes",
		};
	}

	function genSummaryQuestion(item: Media, detail: Media, pool: Media[]): Question | null {
		if (!detail.summary || detail.summary.length < 30) return null;
		const name = item.name ?? "";
		let snippet = detail.summary;
		snippet = snippet.replace(new RegExp(name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'), '???');
		if (snippet.length > 120) snippet = snippet.slice(0, 120).replace(/\s\S*$/, '') + '…';
		const wrongItems = shuffle(pool.filter(m => m.name !== item.name)).slice(0, 3);
		if (wrongItems.length < 3) return null;
		const opts = shuffle([item.name!, ...wrongItems.map(m => m.name!)]);
		return {
			text: `Which item has this description?\n"${snippet}"`,
			options: opts,
			correctIndex: opts.indexOf(item.name!),
			category: "Description",
		};
	}

	function genPosterQuestion(item: Media, pool: Media[]): Question | null {
		const poster = getPoster(item);
		if (!poster) return null;
		const wrongItems = shuffle(pool.filter(m => m.name !== item.name && getPoster(m))).slice(0, 3);
		if (wrongItems.length < 3) return null;
		const opts = shuffle([item.name!, ...wrongItems.map(m => m.name!)]);
		return {
			text: `What is the name of this item?`,
			image: poster,
			options: opts,
			correctIndex: opts.indexOf(item.name!),
			category: "Poster",
		};
	}

	function genWhichHasMoreSeasonsQuestion(items: {item: Media; detail: Media}[]): Question | null {
		const shows = items.filter(i => i.detail.seasons && i.detail.seasons.filter(s => s.number > 0).length > 0);
		if (shows.length < 4) return null;
		const picked = shuffle(shows).slice(0, 4);
		const counts = picked.map(p => ({ name: p.item.name!, count: p.detail.seasons!.filter(s => s.number > 0).length }));
		const unique = new Set(counts.map(c => c.count));
		if (unique.size < 2) return null; // all same
		const max = Math.max(...counts.map(c => c.count));
		const winner = counts.find(c => c.count === max)!;
		if (counts.filter(c => c.count === max).length > 1) return null; // tie
		const opts = counts.map(c => c.name);
		return {
			text: `Which of these has the most seasons?`,
			options: opts,
			correctIndex: opts.indexOf(winner.name),
			category: "Comparison",
		};
	}

	function genWhichIsLongerQuestion(items: {item: Media; detail: Media}[]): Question | null {
		const withRuntime = items.filter(i => i.detail.runtime && i.detail.runtime > 10);
		if (withRuntime.length < 4) return null;
		const picked = shuffle(withRuntime).slice(0, 4);
		const list = picked.map(p => ({ name: p.item.name!, runtime: p.detail.runtime! }));
		const unique = new Set(list.map(c => c.runtime));
		if (unique.size < 2) return null;
		const max = Math.max(...list.map(c => c.runtime));
		const winner = list.find(c => c.runtime === max)!;
		if (list.filter(c => c.runtime === max).length > 1) return null;
		const opts = list.map(c => c.name);
		return {
			text: `Which of these has the longest runtime?`,
			options: opts,
			correctIndex: opts.indexOf(winner.name),
			category: "Comparison",
		};
	}

	function genAnimeQuestion(item: Media, pool: Media[]): Question | null {
		if (!item.isShowAnime) return null;
		const nonAnime = pool.filter(m => !m.isShowAnime && m.type === MediaTypeE.tmdbShow);
		if (nonAnime.length < 3) return null;
		const wrong = shuffle(nonAnime).slice(0, 3);
		const opts = shuffle([item.name!, ...wrong.map(m => m.name!)]);
		return {
			text: `Which of these is an anime?`,
			options: opts,
			correctIndex: opts.indexOf(item.name!),
			category: "Anime",
		};
	}

	function genTypeQuestion(item: Media): Question | null {
		const typeMap: Record<number, string> = {
			[MediaTypeE.tmdbMovie]: "Movie",
			[MediaTypeE.tmdbShow]: "TV Show",
			[MediaTypeE.igdbGame]: "Game",
			[MediaTypeE.malManga]: "Manga",
		};
		if (!item.type || !typeMap[item.type]) return null;
		const correct = typeMap[item.type]!;
		const allTypes = ["Movie", "TV Show", "Game", "Manga"];
		const wrong = allTypes.filter((t) => t !== correct);
		const opts = shuffle([correct, ...wrong.slice(0, 3)]);
		return {
			text: `What type of media is "${item.name}"?`,
			image: getPoster(item),
			options: opts,
			correctIndex: opts.indexOf(correct),
			category: "Media Type",
		};
	}

	async function generateQuestions() {
		loadingQuestions = true;
		const qs: Question[] = [];
		const usedItems = new Set<string>();
		const itemPool = shuffle([...allItems]);

		// Pre-fetch details for a batch of items to enable comparison questions
		const detailCache: {item: Media; detail: Media}[] = [];
		for (const item of itemPool.slice(0, 30)) {
			const detail = await fetchDetails(item);
			if (detail) detailCache.push({item, detail});
		}

		// Single-item generators (no year/rating)
		const easyGens: ((item: Media, detail: Media) => Question | null)[] = [genGenreQuestion, genPosterQuestion as any];
		const mediumGens: ((item: Media, detail: Media) => Question | null)[] = [genSeasonQuestion, genEpisodeCountQuestion, genRuntimeQuestion];
		const hardGens: ((item: Media, detail: Media) => Question | null)[] = [genSummaryQuestion as any];

		// Generate comparison questions first (they use multiple items)
		const comparisonGens = [genWhichHasMoreSeasonsQuestion, genWhichIsLongerQuestion];
		const compQuestions: Question[] = [];
		for (const gen of shuffle(comparisonGens)) {
			const q = gen(detailCache);
			if (q) compQuestions.push(q);
		}

		// Generate per-item questions
		for (const {item, detail} of detailCache) {
			if (qs.length + compQuestions.length >= totalQuestions) break;
			if (usedItems.has(item.name ?? "")) continue;

			const qIndex = qs.length;
			let generators: ((item: Media, detail: Media) => Question | null)[];

			if (qIndex < 4) {
				generators = shuffle([...easyGens]);
			} else if (qIndex < 9) {
				generators = shuffle([...mediumGens]);
			} else {
				generators = shuffle([...hardGens]);
			}

			for (const gen of generators) {
				let q: Question | null = null;
				if (gen === genSummaryQuestion as any) {
					q = genSummaryQuestion(item, detail, allItems);
				} else if (gen === genPosterQuestion as any) {
					q = genPosterQuestion(item, allItems);
				} else if (gen === genAnimeQuestion as any) {
					q = genAnimeQuestion(item, allItems);
				} else {
					q = gen(item, detail);
				}
				if (q) {
					qs.push(q);
					usedItems.add(item.name ?? "");
					break;
				}
			}

			// Fallback: try all generators
			if (!usedItems.has(item.name ?? "")) {
				const allGens = [...easyGens, ...mediumGens, ...hardGens];
				for (const gen of allGens) {
					let q: Question | null = null;
					if (gen === genSummaryQuestion as any) {
						q = genSummaryQuestion(item, detail, allItems);
					} else if (gen === genPosterQuestion as any) {
						q = genPosterQuestion(item, allItems);
					} else {
						q = gen(item, detail);
					}
					if (q) {
						qs.push(q);
						usedItems.add(item.name ?? "");
						break;
					}
				}
			}
		}

		// Try anime questions
		if (qs.length + compQuestions.length < totalQuestions) {
			for (const item of itemPool) {
				if (qs.length + compQuestions.length >= totalQuestions) break;
				if (usedItems.has(item.name ?? "")) continue;
				const q = genAnimeQuestion(item, allItems);
				if (q) {
					qs.push(q);
					usedItems.add(item.name ?? "");
				}
			}
		}

		// Insert comparison questions at medium difficulty positions
		const finalQs: Question[] = [];
		let compIdx = 0;
		for (let i = 0; i < qs.length; i++) {
			finalQs.push(qs[i]);
			if ((i === 4 || i === 7) && compIdx < compQuestions.length) {
				finalQs.push(compQuestions[compIdx++]);
			}
		}
		while (compIdx < compQuestions.length) finalQs.push(compQuestions[compIdx++]);

		questions = finalQs;
		totalQuestions = finalQs.length;
		loadingQuestions = false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) {
			error = "You need at least 4 items in your selection to play.";
			return;
		}
		error = "";
		await generateQuestions();
		if (questions.length < 3) {
			error = "Couldn't generate enough questions. Add more items to your list!";
			return;
		}
		currentQ = 0;
		score = 0;
		streak = 0;
		bestStreak = 0;
		selectedAnswer = null;
		answered = false;
		showCorrect = false;
		lifelines = { fiftyFifty: true, skip: true };
		eliminatedOptions = [];
		gamePhase = "playing";
	}

	function selectAnswer(index: number) {
		if (answered || eliminatedOptions.includes(index)) return;
		selectedAnswer = index;
		answered = true;
		showCorrect = true;

		if (index === questions[currentQ].correctIndex) {
			const basePoints = difficulty === "easy" ? 100 : difficulty === "medium" ? 200 : 400;
			const streakBonus = streak * 50;
			score += basePoints + streakBonus;
			streak++;
			if (streak > bestStreak) bestStreak = streak;
		} else {
			streak = 0;
		}

		scheduleAction(() => {
			if (currentQ + 1 >= questions.length) {
				gamePhase = "result";
			} else {
				currentQ++;
				selectedAnswer = null;
				answered = false;
				showCorrect = false;
				eliminatedOptions = [];
			}
		}, 2000);
	}

	function useFiftyFifty() {
		if (!lifelines.fiftyFifty || answered) return;
		lifelines.fiftyFifty = false;
		const correct = questions[currentQ].correctIndex;
		const wrongIndices = [0, 1, 2, 3].filter((i) => i !== correct);
		const toEliminate = pickRandom(wrongIndices, 2);
		eliminatedOptions = toEliminate;
	}

	function useSkip() {
		if (!lifelines.skip || answered) return;
		lifelines.skip = false;
		if (currentQ + 1 >= questions.length) {
			gamePhase = "result";
		} else {
			currentQ++;
			selectedAnswer = null;
			answered = false;
			showCorrect = false;
			eliminatedOptions = [];
		}
	}

	function getScoreEmoji(pct: number): string {
		if (pct >= 90) return "🏆";
		if (pct >= 70) return "🌟";
		if (pct >= 50) return "👍";
		if (pct >= 30) return "🤔";
		return "💀";
	}

	$effect(() => {
		if (gamePhase === "result") {
			axios.post("/gamescore", { game: "trivia", score, bestStreak }).catch(() => {});
		}
	});

	onMount(() => {
		fetchStatusCounts();
		loadItems();
	});
</script>

<svelte:head>
	<title>Trivia Quiz</title>
</svelte:head>

<div class="trivia-page" onclick={skipResult}>
	<PageTitle title="Trivia Quiz" />

	{#if gamePhase === "setup"}
		<p class="subtitle">How well do you <em>really</em> know your watchlist?</p>

		<!-- Filter mode toggle -->
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
				{#if !tiersLoaded}
					<SpinnerTiny />
				{:else if tiers.length === 0}
					<span class="no-tiers">No tiers found.</span>
				{:else}
					{#each tiers as tier}
						<button class="plain filter-btn tier-filter-btn" class:active={enabledTierIds.includes(tier.id)} onclick={() => toggleTier(tier.id)} style="--tier-bg: {tier.color}; --tier-text: {tier.textColor};">
							{tier.name}{tier.tierItems ? ` (${tier.tierItems.length})` : ""}
						</button>
					{/each}
				{/if}
			</div>
		{/if}

		{#if error}
			<div class="error-msg">{error}</div>
		{/if}

		<button class="plain start-btn" onclick={startGame} disabled={loadingQuestions || loading}>
			{#if loadingQuestions}
				<SpinnerTiny /> Generating Questions...
			{:else if loading}
				<SpinnerTiny /> Loading...
			{:else}
				<Icon i="sparkles" wh={22} />
				Start Quiz
			{/if}
		</button>

	{:else if gamePhase === "playing"}
		{@const q = questions[currentQ]}

		<!-- Progress bar -->
		<div class="progress-bar">
			<div class="progress-fill" style="width: {((currentQ) / questions.length) * 100}%"></div>
		</div>

		<div class="game-header">
			<div class="game-stat">
				<span class="stat-label">Question</span>
				<span class="stat-value">{currentQ + 1}/{questions.length}</span>
			</div>
			<div class="game-stat">
				<span class="stat-label">Score</span>
				<span class="stat-value">{score.toLocaleString()}</span>
			</div>
			<div class="game-stat">
				<span class="stat-label">Streak</span>
				<span class="stat-value">🔥 {streak}</span>
			</div>
			<div class="game-stat difficulty-badge" class:easy={difficulty === "easy"} class:medium={difficulty === "medium"} class:hard={difficulty === "hard"}>
				<span>{difficulty.toUpperCase()}</span>
			</div>
		</div>

		<!-- Prize ladder -->
		<div class="prize-ladder">
			{#each prizeLabels as prize, i}
				<div class="prize-step" class:current={i === currentQ} class:done={i < currentQ} class:future={i > currentQ}>
					{prize}
				</div>
			{/each}
		</div>

		<div class="question-card">
			<span class="q-category">{q.category}</span>
			{#if q.image}
				<img class="q-image" src={q.image} alt="?" />
			{/if}
			<h2 class="q-text">{q.text}</h2>
		</div>

		<div class="options-grid">
			{#each q.options as opt, i}
				{@const letter = ["A", "B", "C", "D"][i]}
				<button
					class="plain option-btn"
					class:correct={showCorrect && i === q.correctIndex}
					class:wrong={showCorrect && selectedAnswer === i && i !== q.correctIndex}
					class:eliminated={eliminatedOptions.includes(i)}
					disabled={answered || eliminatedOptions.includes(i)}
					onclick={() => selectAnswer(i)}
				>
					<span class="option-letter">{letter}</span>
					<span class="option-text">{opt}</span>
				</button>
			{/each}
		</div>

		<div class="lifelines">
			<button class="plain lifeline-btn" disabled={!lifelines.fiftyFifty || answered} onclick={useFiftyFifty}>
				50:50
			</button>
			<button class="plain lifeline-btn" disabled={!lifelines.skip || answered} onclick={useSkip}>
				⏭ Skip
			</button>
		</div>

	{:else if gamePhase === "result"}
		{@const correctCount = score > 0 ? Math.round(score / 150) : 0}
		{@const pct = Math.round((correctCount / questions.length) * 100)}

		<div class="result-card">
			<div class="result-emoji">{getScoreEmoji(pct)}</div>
			<h2>Quiz Complete!</h2>
			<div class="result-stats">
				<div class="result-stat">
					<span class="result-stat-value">{score.toLocaleString()}</span>
					<span class="result-stat-label">Total Score</span>
				</div>
				<div class="result-stat">
					<span class="result-stat-value">🔥 {bestStreak}</span>
					<span class="result-stat-label">Best Streak</span>
				</div>
				<div class="result-stat">
					<span class="result-stat-value">{questions.length}</span>
					<span class="result-stat-label">Questions</span>
				</div>
			</div>
			<div class="result-actions">
				<button class="plain start-btn" onclick={startGame}>
					<Icon i="refresh" wh={18} />
					Play Again
				</button>
				<a href="/games" class="plain back-link">Back to Games</a>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
	.trivia-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		padding: 20px;
		width: 100%;
		max-width: 700px;
		margin: 0 auto;
	}

	.subtitle {
		font-size: 15px;
		color: $text-color-accent;
		margin: 0;

		em { font-style: italic; }
	}

	.filter-mode-toggle {
		display: flex;
		gap: 4px;
		background: $accent-color;
		border-radius: 10px;
		padding: 3px;
	}

	.filter-mode-btn {
		padding: 6px 14px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: $text-color-accent;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
		&.active { background: $accent-color-hover; color: $bg-color; }
		&:hover:not(.active) { color: $text-color; }
	}

	.picker-filters {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: center;
	}

	.filter-btn {
		padding: 8px 14px;
		border-radius: 8px;
		border: 2px solid $text-color;
		background: $bg-color;
		color: $text-color;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
		&:hover, &.active { background: $accent-color-hover; color: $bg-color; }
		&.active { border-color: $bg-color; }
	}

	.tier-filter-btn {
		border-color: var(--tier-bg);
		&:hover, &.active { background: var(--tier-bg); color: var(--tier-text); border-color: var(--tier-bg); }
	}

	.no-tiers { font-size: 13px; color: $text-color-accent; }

	.error-msg {
		color: $error;
		font-size: 14px;
		text-align: center;
	}

	.start-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 14px 36px;
		border-radius: 10px;
		border: 2px solid $text-color;
		background: $bg-color;
		color: $text-color;
		fill: $text-color;
		font-size: 18px;
		font-weight: 700;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
		&:hover:not(:disabled) { background: $text-color; color: $bg-color; fill: $bg-color; }
		&:disabled { opacity: 0.5; cursor: not-allowed; }
	}

	// --- Playing phase ---

	.progress-bar {
		width: 100%;
		height: 6px;
		background: $accent-color;
		border-radius: 3px;
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: $accent-color-hover;
		border-radius: 3px;
		transition: width 400ms ease;
	}

	.game-header {
		display: flex;
		gap: 16px;
		align-items: center;
		flex-wrap: wrap;
		justify-content: center;
		width: 100%;
	}

	.game-stat {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
	}

	.stat-label {
		font-size: 10px;
		text-transform: uppercase;
		color: $text-color-accent;
		font-weight: 600;
	}

	.stat-value {
		font-size: 18px;
		font-weight: 700;
		color: $text-color;
	}

	.difficulty-badge {
		padding: 4px 12px;
		border-radius: 8px;
		font-size: 11px;
		font-weight: 700;

		span { color: white; }

		&.easy { background: #2ecc71; }
		&.medium { background: #f39c12; }
		&.hard { background: #e74c3c; }
	}

	.prize-ladder {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
		justify-content: center;
		width: 100%;
	}

	.prize-step {
		font-size: 11px;
		font-weight: 600;
		padding: 3px 8px;
		border-radius: 6px;
		background: $accent-color;
		color: $text-color-accent;
		transition: all 200ms ease;

		&.current { background: $accent-color-hover; color: $bg-color; transform: scale(1.1); }
		&.done { background: $bg-color-accent; color: $text-color-accent; opacity: 0.5; }
	}

	.question-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
		padding: 24px;
		border-radius: 16px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		width: 100%;
	}

	.q-category {
		font-size: 11px;
		font-weight: 600;
		padding: 4px 10px;
		border-radius: 6px;
		background: $bg-color-accent;
		color: $text-color-accent;
		text-transform: uppercase;
	}

	.q-image {
		width: 100px;
		height: 140px;
		object-fit: cover;
		border-radius: 10px;
	}

	.q-text {
		margin: 0;
		font-size: 18px;
		font-weight: 600;
		text-align: center;
		color: $text-color;
		line-height: 1.4;
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
		font-size: 15px;
		font-weight: 600;
		cursor: pointer;
		transition: all 200ms ease;
		text-align: left;

		&:hover:not(:disabled):not(.eliminated) {
			border-color: $accent-color-hover;
			background: $bg-color-accent;
		}

		&.correct {
			border-color: #2ecc71;
			background: rgba(46, 204, 113, 0.2);
			color: #2ecc71;
		}

		&.wrong {
			border-color: #e74c3c;
			background: rgba(231, 76, 60, 0.2);
			color: #e74c3c;
		}

		&.eliminated {
			opacity: 0.2;
			pointer-events: none;
		}

		&:disabled:not(.correct):not(.wrong):not(.eliminated) {
			opacity: 0.7;
			cursor: default;
		}
	}

	.option-letter {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 8px;
		background: $bg-color-accent;
		font-size: 13px;
		font-weight: 700;
		flex-shrink: 0;
	}

	.option-text {
		flex: 1;
	}

	.lifelines {
		display: flex;
		gap: 10px;
	}

	.lifeline-btn {
		padding: 8px 20px;
		border-radius: 8px;
		border: 2px solid $bg-color-accent;
		background: $accent-color;
		color: $text-color;
		font-size: 14px;
		font-weight: 700;
		cursor: pointer;
		transition: all 150ms ease;

		&:hover:not(:disabled) { border-color: $accent-color-hover; }
		&:disabled { opacity: 0.3; cursor: not-allowed; }
	}

	// --- Result ---

	.result-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		padding: 32px;
		border-radius: 20px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		width: 100%;

		h2 {
			margin: 0;
			font-size: 24px;
			font-weight: 700;
		}
	}

	.result-emoji {
		font-size: 64px;
		animation: result-bounce 500ms ease;
	}

	@keyframes result-bounce {
		0% { transform: scale(0); }
		50% { transform: scale(1.3); }
		100% { transform: scale(1); }
	}

	.result-stats {
		display: flex;
		gap: 24px;
		flex-wrap: wrap;
		justify-content: center;
	}

	.result-stat {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
	}

	.result-stat-value {
		font-size: 24px;
		font-weight: 700;
		color: $text-color;
	}

	.result-stat-label {
		font-size: 12px;
		color: $text-color-accent;
		font-weight: 500;
	}

	.result-actions {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		width: 100%;
	}

	.back-link {
		font-size: 14px;
		color: $text-color-accent;
		text-decoration: none;
		&:hover { color: $text-color; }
	}
</style>
