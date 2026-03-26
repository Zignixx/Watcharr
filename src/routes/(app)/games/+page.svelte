<script lang="ts">
	import { onMount } from "svelte";
	import axios from "axios";
	import PageTitle from "@/lib/generic/PageTitle.svelte";
	import Icon from "@/lib/Icon.svelte";

	type GameScore = { game: string; highScore: number; bestStreak: number; timesPlayed: number };
	let scores: Record<string, GameScore> = $state({});

	onMount(async () => {
		try {
			const r = await axios.get<GameScore[]>("/gamescore");
			for (const s of r.data ?? []) scores[s.game] = s;
		} catch {}
	});
</script>

<svelte:head>
	<title>Mini Games</title>
</svelte:head>

<div class="games-hub">
	<PageTitle title="Mini Games" />

	<div class="games-grid">
		<a href="/games/trivia" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #667eea, #764ba2);">
				<Icon i="sparkles" wh={40} />
			</div>
			<h3>Trivia Quiz</h3>
			<p>How well do you really know your watchlist? Answer questions about release years, seasons, genres and more!</p>
			{#if scores.trivia}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.trivia.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.trivia.bestStreak}</span>
					<span class="score-badge">🎮 {scores.trivia.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Wer Wird Millionär</span>
		</a>

		<a href="/games/highlow" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #f093fb, #f5576c);">
				<Icon i="sort" wh={40} />
			</div>
			<h3>Higher or Lower</h3>
			<p>Is this rated higher or lower? Compare ratings from your watchlist and build the longest streak!</p>
			{#if scores.highlow}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.highlow.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.highlow.bestStreak}</span>
					<span class="score-badge">🎮 {scores.highlow.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Streak Mode</span>
		</a>

		<a href="/games/poster" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #4facfe, #00f2fe);">
				<Icon i="eye-closed" wh={40} />
			</div>
			<h3>Guess the Poster</h3>
			<p>Can you recognize your watchlist items from a pixelated poster? The image gets clearer with each wrong guess!</p>
			{#if scores.poster}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.poster.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.poster.bestStreak}</span>
					<span class="score-badge">🎮 {scores.poster.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Visual Challenge</span>
		</a>

		<a href="/games/emoji" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #f7971e, #ffd200);">
				<Icon i="document" wh={40} />
			</div>
			<h3>Plot Twist</h3>
			<p>Read a plot description and guess which item from your watchlist it belongs to!</p>
			{#if scores.emoji}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.emoji.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.emoji.bestStreak}</span>
					<span class="score-badge">🎮 {scores.emoji.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Story Detective</span>
		</a>

		<a href="/games/timeline" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #a18cd1, #fbc2eb);">
				<Icon i="calendar" wh={40} />
			</div>
			<h3>Release Timeline</h3>
			<p>Sort 4 items from oldest to newest by release date. Drag or use arrows to reorder!</p>
			{#if scores.timeline}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.timeline.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.timeline.bestStreak}</span>
					<span class="score-badge">🎮 {scores.timeline.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Sort Challenge</span>
		</a>

		<a href="/games/episodes" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #ff9a9e, #fecfef);">
				<Icon i="film" wh={40} />
			</div>
			<h3>Name That Show</h3>
			<p>Read 2-3 episode titles and guess which TV show they belong to!</p>
			{#if scores.episodes}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.episodes.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.episodes.bestStreak}</span>
					<span class="score-badge">🎮 {scores.episodes.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Episode Expert</span>
		</a>

		<a href="/games/epcount" class="game-card">
			<div class="game-icon" style="background: linear-gradient(135deg, #43e97b, #38f9d7);">
				<Icon i="tv" wh={40} />
			</div>
			<h3>Episode Counter</h3>
			<p>See the total episode count and number of seasons — can you guess which show it is?</p>
			{#if scores.epcount}
				<div class="score-row">
					<span class="score-badge">🏆 {scores.epcount.highScore.toLocaleString()}</span>
					<span class="score-badge">🔥 {scores.epcount.bestStreak}</span>
					<span class="score-badge">🎮 {scores.epcount.timesPlayed}x</span>
				</div>
			{/if}
			<span class="game-tag">Number Cruncher</span>
		</a>
	</div>
</div>

<style lang="scss">
	.games-hub {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 30px;
		padding: 20px;
		width: 100%;
		max-width: 900px;
		margin: 0 auto;
	}

	.games-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 20px;
		width: 100%;
	}

	.game-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
		padding: 28px 20px;
		border-radius: 16px;
		background: $accent-color;
		border: 1px solid $bg-color-accent;
		text-decoration: none;
		color: $text-color;
		fill: $text-color;
		transition: transform 150ms ease, border-color 150ms ease, box-shadow 150ms ease;

		&:hover {
			transform: translateY(-4px);
			border-color: $accent-color-hover;
			box-shadow: 0 8px 30px rgba(0, 0, 0, 0.3);
		}

		h3 {
			margin: 0;
			font-size: 18px;
			font-weight: 700;
		}

		p {
			margin: 0;
			font-size: 13px;
			color: $text-color-accent;
			text-align: center;
			line-height: 1.5;
		}
	}

	.game-icon {
		width: 80px;
		height: 80px;
		border-radius: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
		fill: white;
	}

	.game-tag {
		font-size: 11px;
		font-weight: 600;
		padding: 4px 10px;
		border-radius: 6px;
		background: $bg-color-accent;
		color: $text-color-accent;
	}

	.score-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: center;
	}

	.score-badge {
		font-size: 12px;
		font-weight: 600;
		padding: 3px 8px;
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.08);
		color: $text-color;
	}
</style>
