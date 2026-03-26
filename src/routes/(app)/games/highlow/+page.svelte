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
	type CompareMode = "rating" | "year";

	// --- Filter state ---
	let filterMode: FilterMode = $state("status");
	let enabledStatuses: WatchedStatus[] = $state(["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"]);
	let tiers: Tier[] = $state([]);
	let enabledTierIds: number[] = $state([]);
	let tiersLoaded = $state(false);
	let statusCounts: Record<string, number> = $state({});

	// --- Game state ---
	let allItems: Media[] = $state([]);
	let detailCache: Map<string, Media> = new Map();
	let loading = $state(true);
	let error = $state("");
	let gamePhase: "setup" | "playing" | "gameover" = $state("setup");
	let compareMode: CompareMode = $state("rating");

	let leftItem: Media | null = $state(null);
	let rightItem: Media | null = $state(null);
	let leftDetail: Media | null = $state(null);
	let rightDetail: Media | null = $state(null);
	let leftValue: number = $state(0);
	let rightValue: number = $state(0);
	let revealed = $state(false);
	let wasCorrect = $state(false);
	let streak = $state(0);
	let bestStreak = $state(0);
	let score = $state(0);
	let animating = $state(false);

	function toggleStatus(s: WatchedStatus) {
		if (enabledStatuses.includes(s)) {
			if (enabledStatuses.length <= 1) return;
			enabledStatuses = enabledStatuses.filter((x) => x !== s);
		} else { enabledStatuses = [...enabledStatuses, s]; }
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
		} else { enabledTierIds = [...enabledTierIds, id]; }
	}

	function switchFilterMode(m: FilterMode) {
		if (filterMode === m) return; filterMode = m;
		if (m === "tier") loadTiers();
	}

	async function fetchStatusCounts() {
		const statuses: WatchedStatus[] = ["PLANNED", "WATCHING", "HOLD", "FINISHED", "DROPPED"];
		const counts: Record<string, number> = {};
		await Promise.all(statuses.map(async (s) => {
			try {
				const r = await axios.get("/watched", { params: { status: s, limit: 1, page: 1 } });
				counts[s] = r.data?.totalResults ?? 0;
			} catch { counts[s] = 0; }
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
			return `https://image.tmdb.org/t/p/w200${m.extPosterPath}`;
		}
		if (m.type === MediaTypeE.igdbGame) return `https://images.igdb.com/igdb/image/upload/t_cover_big/${m.extPosterPath}.jpg`;
		if (m.type === MediaTypeE.malManga) return m.extPosterPath;
		return "";
	}

	function getItemKey(m: Media): string {
		return `${m.type}-${m.ids.tmdb || m.ids.igdb || m.ids.mal}`;
	}

	async function fetchDetails(m: Media): Promise<Media | null> {
		const key = getItemKey(m);
		if (detailCache.has(key)) return detailCache.get(key)!;
		try {
			let r;
			if (m.type === MediaTypeE.tmdbMovie) r = await axios.get(`/content/movie/${m.ids.tmdb}`);
			else if (m.type === MediaTypeE.tmdbShow) r = await axios.get(`/content/tv/${m.ids.tmdb}`);
			else if (m.type === MediaTypeE.igdbGame) r = await axios.get(`/game/${m.ids.igdb}`);
			else return null;
			detailCache.set(key, r!.data);
			return r!.data;
		} catch { return null; }
	}

	function getValue(detail: Media): number {
		if (compareMode === "rating") {
			const r = detail.rating ?? 0;
			return Math.round(r) / 10; // TMDB 100 scale -> 10 scale
		} else {
			if (!detail.releaseDate) return 0;
			return new Date(detail.releaseDate).getFullYear();
		}
	}

	function formatValue(val: number): string {
		if (compareMode === "rating") return val.toFixed(1);
		return String(val);
	}

	function shuffle<T>(arr: T[]): T[] {
		const a = [...arr];
		for (let i = a.length - 1; i > 0; i--) {
			const j = Math.floor(Math.random() * (i + 1));
			[a[i], a[j]] = [a[j], a[i]];
		}
		return a;
	}

	async function pickNewPair() {
		const pool = shuffle(allItems);
		for (let i = 0; i < pool.length - 1; i++) {
			for (let j = i + 1; j < pool.length; j++) {
				const d1 = await fetchDetails(pool[i]);
				const d2 = await fetchDetails(pool[j]);
				if (!d1 || !d2) continue;
				const v1 = getValue(d1);
				const v2 = getValue(d2);
				if (v1 === 0 || v2 === 0 || v1 === v2) continue;
				leftItem = pool[i]; rightItem = pool[j];
				leftDetail = d1; rightDetail = d2;
				leftValue = v1; rightValue = v2;
				return true;
			}
		}
		return false;
	}

	async function startGame() {
		await loadItems();
		if (allItems.length < 4) { error = "You need at least 4 items."; return; }
		error = "";
		streak = 0; bestStreak = 0; score = 0;
		revealed = false; animating = false;
		detailCache.clear();
		const ok = await pickNewPair();
		if (!ok) { error = "Couldn't find items with valid data. Try different filters!"; return; }
		gamePhase = "playing";
	}

	async function guess(choice: "higher" | "lower") {
		if (revealed || animating) return;
		revealed = true;
		animating = true;

		const isHigher = rightValue > leftValue;
		wasCorrect = (choice === "higher" && isHigher) || (choice === "lower" && !isHigher);

		if (wasCorrect) {
			streak++;
			score += 100 + streak * 25;
			if (streak > bestStreak) bestStreak = streak;

			setTimeout(async () => {
				// Shift right to left, pick new right
				leftItem = rightItem;
				leftDetail = rightDetail;
				leftValue = rightValue;

				const pool = shuffle(allItems).filter((m) => getItemKey(m) !== getItemKey(leftItem!));
				let found = false;
				for (const m of pool) {
					const d = await fetchDetails(m);
					if (!d) continue;
					const v = getValue(d);
					if (v === 0 || v === leftValue) continue;
					rightItem = m; rightDetail = d; rightValue = v;
					found = true;
					break;
				}
				if (!found) { gamePhase = "gameover"; return; }
				revealed = false;
				animating = false;
			}, 1500);
		} else {
			setTimeout(() => { gamePhase = "gameover"; }, 2000);
		}
	}

	onMount(() => { fetchStatusCounts(); loadItems(); });

	$effect(() => {
		if (gamePhase === "gameover") {
			axios.post("/gamescore", { game: "highlow", score, bestStreak }).catch(() => {});
		}
	});
</script>

<svelte:head>
	<title>Higher or Lower</title>
</svelte:head>

<div class="hl-page">
	<PageTitle title="Higher or Lower" />

	{#if gamePhase === "setup"}
		<p class="subtitle">Is it rated higher or lower? Build the longest streak!</p>

		<!-- Compare mode -->
		<div class="compare-toggle">
			<button class="plain compare-btn" class:active={compareMode === "rating"} onclick={() => { compareMode = "rating"; }}>
				<Icon i="star" wh={16} /> Rating
			</button>
			<button class="plain compare-btn" class:active={compareMode === "year"} onclick={() => { compareMode = "year"; }}>
				<Icon i="calendar" wh={16} /> Release Year
			</button>
		</div>

		<!-- Filter mode -->
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

		{#if error}<div class="error-msg">{error}</div>{/if}

		<button class="plain start-btn" onclick={startGame} disabled={loading}>
			{#if loading}<SpinnerTiny /> Loading...
			{:else}<Icon i="sort" wh={22} /> Start Game{/if}
		</button>

	{:else if gamePhase === "playing"}
		<div class="game-header">
			<div class="game-stat"><span class="stat-label">Streak</span><span class="stat-value">🔥 {streak}</span></div>
			<div class="game-stat"><span class="stat-label">Score</span><span class="stat-value">{score.toLocaleString()}</span></div>
			<div class="game-stat"><span class="stat-label">Comparing</span><span class="stat-value">{compareMode === "rating" ? "⭐ Rating" : "📅 Year"}</span></div>
		</div>

		<div class="hl-arena">
			<!-- Left card (known) -->
			<div class="hl-card known">
				{#if leftItem}
					{@const poster = getPoster(leftItem)}
					{#if poster}
						<img class="hl-poster" src={poster} alt={leftItem.name} />
					{/if}
					<h3>{leftItem.name}</h3>
					<div class="hl-value">{formatValue(leftValue)}</div>
					<span class="hl-value-label">{compareMode === "rating" ? "Rating" : "Released"}</span>
				{/if}
			</div>

			<!-- VS divider -->
			<div class="vs-divider">
				<span>VS</span>
			</div>

			<!-- Right card (guess) -->
			<div class="hl-card mystery" class:correct={revealed && wasCorrect} class:wrong={revealed && !wasCorrect}>
				{#if rightItem}
					{@const poster = getPoster(rightItem)}
					{#if poster}
						<img class="hl-poster" src={poster} alt={rightItem.name} />
					{/if}
					<h3>{rightItem.name}</h3>

					{#if revealed}
						<div class="hl-value reveal-anim">{formatValue(rightValue)}</div>
						<span class="hl-value-label">{compareMode === "rating" ? "Rating" : "Released"}</span>
						<div class="hl-verdict">{wasCorrect ? "✅ Correct!" : "❌ Wrong!"}</div>
					{:else}
						<div class="hl-value">?</div>
						<div class="guess-buttons">
							<button class="plain guess-btn higher" onclick={() => guess("higher")}>
								▲ Higher
							</button>
							<button class="plain guess-btn lower" onclick={() => guess("lower")}>
								▼ Lower
							</button>
						</div>
					{/if}
				{/if}
			</div>
		</div>

	{:else if gamePhase === "gameover"}
		<div class="result-card">
			<div class="result-emoji">{bestStreak >= 10 ? "🏆" : bestStreak >= 5 ? "🌟" : bestStreak >= 3 ? "👍" : "💀"}</div>
			<h2>Game Over!</h2>
			<div class="result-stats">
				<div class="result-stat">
					<span class="result-stat-value">{score.toLocaleString()}</span>
					<span class="result-stat-label">Total Score</span>
				</div>
				<div class="result-stat">
					<span class="result-stat-value">🔥 {bestStreak}</span>
					<span class="result-stat-label">Best Streak</span>
				</div>
			</div>
			<div class="result-actions">
				<button class="plain start-btn" onclick={startGame}><Icon i="refresh" wh={18} /> Play Again</button>
				<a href="/games" class="plain back-link">Back to Games</a>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
	.hl-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		padding: 20px;
		width: 100%;
		max-width: 800px;
		margin: 0 auto;
	}

	.subtitle { font-size: 15px; color: $text-color-accent; margin: 0; }

	.compare-toggle {
		display: flex;
		gap: 4px;
		background: $accent-color;
		border-radius: 10px;
		padding: 3px;
	}

	.compare-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: $text-color-accent;
		fill: $text-color-accent;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: all 150ms ease;
		&.active { background: $accent-color-hover; color: $bg-color; fill: $bg-color; }
		&:hover:not(.active) { color: $text-color; fill: $text-color; }
	}

	.filter-mode-toggle {
		display: flex; gap: 4px; background: $accent-color; border-radius: 10px; padding: 3px;
	}

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

	.game-header {
		display: flex; gap: 20px; align-items: center; flex-wrap: wrap; justify-content: center;
	}

	.game-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
	.stat-label { font-size: 10px; text-transform: uppercase; color: $text-color-accent; font-weight: 600; }
	.stat-value { font-size: 18px; font-weight: 700; color: $text-color; }

	// --- Arena ---
	.hl-arena {
		display: flex;
		align-items: stretch;
		gap: 0;
		width: 100%;
		min-height: 380px;
	}

	.hl-card {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 24px 16px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		transition: all 300ms ease;

		&.known {
			border-radius: 16px 0 0 16px;
		}

		&.mystery {
			border-radius: 0 16px 16px 0;
			border-left: none;
		}

		&.correct { border-color: #2ecc71; background: rgba(46, 204, 113, 0.1); }
		&.wrong { border-color: #e74c3c; background: rgba(231, 76, 60, 0.1); }

		h3 {
			margin: 0;
			font-size: 16px;
			font-weight: 700;
			text-align: center;
			color: $text-color;
		}
	}

	.hl-poster {
		width: 100px;
		height: 140px;
		object-fit: cover;
		border-radius: 10px;
		box-shadow: 0 4px 15px rgba(0,0,0,0.3);
	}

	.hl-value {
		font-size: 32px;
		font-weight: 800;
		color: $accent-color-hover;
	}

	.hl-value-label {
		font-size: 11px;
		color: $text-color-accent;
		text-transform: uppercase;
		font-weight: 600;
	}

	.reveal-anim {
		animation: value-pop 400ms ease;
	}

	@keyframes value-pop {
		0% { transform: scale(0.5); opacity: 0; }
		50% { transform: scale(1.2); }
		100% { transform: scale(1); opacity: 1; }
	}

	.vs-divider {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 50px;
		flex-shrink: 0;
		background: $bg-color-accent;
		z-index: 1;
		margin: 0 -1px;

		span {
			font-size: 18px;
			font-weight: 800;
			color: $text-color;
			writing-mode: vertical-lr;
		}
	}

	.guess-buttons {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
		max-width: 160px;
	}

	.guess-btn {
		padding: 12px;
		border-radius: 10px;
		border: 2px solid $bg-color-accent;
		background: $accent-color;
		color: $text-color;
		font-size: 16px;
		font-weight: 700;
		cursor: pointer;
		transition: all 150ms ease;

		&.higher:hover { border-color: #2ecc71; color: #2ecc71; background: rgba(46,204,113,0.1); }
		&.lower:hover { border-color: #e74c3c; color: #e74c3c; background: rgba(231,76,60,0.1); }
	}

	.hl-verdict {
		font-size: 18px;
		font-weight: 700;
		animation: value-pop 300ms ease;
	}

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
