<script lang="ts">
	import {
		type WatchedStatus,
		type Watched,
		type Media,
		type SupportedMedia,
		MediaTypeE,
	} from "@/types";
	import { updateWatched, removeWatched, baseURL } from "../util/api";
	import { notify } from "../util/notify";
	import Icon from "../Icon.svelte";
	import { watchedStatuses, toUnderstandableStatus } from "../util/helpers";
	import Rating from "../rating/Rating.svelte";
	import { toShowableRating } from "../rating/helpers";
	import { goto } from "$app/navigation";

	interface Props {
		media: Media;
		watched: Watched | undefined;
		contentId: number;
		contentType: SupportedMedia;
		poster: string | undefined;
		onClose: () => void;
		onWatchedUpdate: (w: Watched | undefined) => void;
		ownerWatched?: Watched;
		ownerName?: string;
	}

	let {
		media,
		watched,
		contentId,
		contentType,
		poster,
		onClose,
		onWatchedUpdate,
		ownerWatched,
		ownerName,
	}: Props = $props();

	let thoughtsOpen = $state(!!watched?.thoughts);
	let thoughtsText = $state(watched?.thoughts ?? "");
	let textarea: HTMLTextAreaElement | undefined = $state();
	let saving = $state(false);
	let modalEl: HTMLDivElement | undefined = $state();

	const year = $derived(
		media.releaseDate ? new Date(media.releaseDate).getFullYear() : undefined,
	);

	const link = $derived.by(() => {
		let id: number | undefined;
		let type: string;
		switch (media.type) {
			case MediaTypeE.tmdbMovie:
				id = media.ids.tmdb;
				type = "movie";
				break;
			case MediaTypeE.tmdbShow:
				id = media.ids.tmdb;
				type = "tv";
				break;
			case MediaTypeE.igdbGame:
				id = media.ids.igdb;
				type = "game";
				break;
			default:
				return;
		}
		return id ? `/${type}/${id}` : undefined;
	});

	const isForGame = $derived(contentType === "game");

	async function handleStatusClick(status: WatchedStatus | "DELETE") {
		if (status === "DELETE") {
			if (!watched) return;
			saving = true;
			const removed = await removeWatched(watched.id);
			if (removed) {
				onWatchedUpdate(undefined);
			}
			saving = false;
			return;
		}
		if (status === watched?.status) return;
		saving = true;
		try {
			const w = await updateWatched(watched, {
				contentId,
				contentType,
				status,
			});
			onWatchedUpdate(w);
		} catch {}
		saving = false;
	}

	async function handleRatingChange(newRating: number) {
		if (newRating === watched?.rating) return;
		saving = true;
		try {
			const w = await updateWatched(watched, {
				contentId,
				contentType,
				rating: newRating,
			});
			onWatchedUpdate(w);
		} catch {}
		saving = false;
	}

	async function handleThoughtsSave() {
		if (thoughtsText === (watched?.thoughts ?? "")) {
			thoughtsOpen = false;
			return;
		}
		saving = true;
		try {
			let w = watched;
			if (!w?.id) {
				w = await updateWatched(undefined, {
					contentId,
					contentType,
					status: "PLANNED" as WatchedStatus,
				});
				if (w) onWatchedUpdate(w);
			}
			if (w?.id) {
				const updated = await updateWatched(w, {
					contentId,
					contentType,
					thoughts: thoughtsText,
				});
				onWatchedUpdate(updated);
			}
			thoughtsOpen = false;
		} catch {}
		saving = false;
	}

	function resizeTextarea() {
		if (!textarea) return;
		textarea.style.height = "";
		textarea.style.height = textarea.scrollHeight + "px";
	}

	function handleBackdropClick(e: MouseEvent | TouchEvent) {
		if (modalEl && !modalEl.contains(e.target as Node)) {
			onClose();
		}
	}

	function navigateToDetail() {
		if (link) {
			onClose();
			goto(link);
		}
	}

	$effect(() => {
		if (textarea) resizeTextarea();
	});
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="mpm-backdrop" onclick={handleBackdropClick}>
	<div class="mpm-modal" bind:this={modalEl}>
		<!-- Header with poster + title -->
		<div class="mpm-header">
			{#if poster}
				<img class="mpm-poster" src={poster} alt="" />
			{/if}
			<div class="mpm-title-area">
				<h2 class="mpm-title">
					{media.name}
					{#if year}
						<time>{year}</time>
					{/if}
				</h2>
				{#if link}
					<button class="plain mpm-detail-link" onclick={navigateToDetail}>
						View details →
					</button>
				{/if}
			</div>
			<button class="plain mpm-close" onclick={onClose}>
				<Icon i="close" wh={22} />
			</button>
		</div>

		<div class="mpm-body small-scrollbar">
			<!-- Owner info (read-only) -->
			{#if ownerWatched}
				<div class="mpm-section mpm-owner-section">
					<span class="mpm-label">{ownerName ? `${ownerName}'s Info` : "Owner's Info"}</span>
					<div class="mpm-owner-row">
						{#if ownerWatched.status}
							<span class="mpm-owner-badge">
								<Icon i={watchedStatuses[ownerWatched.status]} wh={14} />
								{toUnderstandableStatus(ownerWatched.status, isForGame)}
							</span>
						{/if}
						{#if ownerWatched.rating}
							<span class="mpm-owner-badge">★ {toShowableRating(ownerWatched.rating)}</span>
						{/if}
					</div>
					{#if ownerWatched.thoughts}
						<div class="mpm-owner-thoughts">"{ownerWatched.thoughts}"</div>
					{/if}
				</div>
			{/if}

			<!-- Status Section -->
			<div class="mpm-section">
				<span class="mpm-label">Status</span>
				<div class="mpm-statuses">
					{#each Object.entries(watchedStatuses) as [statusName, icon]}
						<button
							class="mpm-status-btn"
							class:active={watched?.status === statusName}
							onclick={() => handleStatusClick(statusName as WatchedStatus)}
							disabled={saving}
						>
							<Icon i={icon} wh={18} />
							<span>{toUnderstandableStatus(statusName as WatchedStatus, isForGame)}</span>
						</button>
					{/each}
					{#if watched}
						<button
							class="mpm-status-btn delete"
							onclick={() => handleStatusClick("DELETE")}
							disabled={saving}
						>
							<Icon i="trash" wh={18} />
							<span>Remove</span>
						</button>
					{/if}
				</div>
			</div>

			<!-- Rating Section -->
			<div class="mpm-section">
				<span class="mpm-label">Rating</span>
				<div class="mpm-rating">
					<Rating rating={watched?.rating} onChange={handleRatingChange} />
				</div>
			</div>

			<!-- Thoughts Section -->
			<div class="mpm-section">
				{#if !thoughtsOpen}
					<button
						class="plain mpm-thoughts-btn"
						onclick={() => { thoughtsOpen = true; }}
					>
						<Icon i="pencil" wh={18} />
						<span>
							{#if watched?.thoughts}
								Edit thoughts
							{:else}
								Add thoughts
							{/if}
						</span>
					</button>
				{:else}
					<div class="mpm-thoughts-editor">
						<textarea
							bind:this={textarea}
							bind:value={thoughtsText}
							oninput={resizeTextarea}
							placeholder="Your thoughts on {media.name}"
							rows="3"
						></textarea>
						<div class="mpm-thoughts-actions">
							<button class="mpm-save-btn" onclick={handleThoughtsSave} disabled={saving}>
								Save
							</button>
							<button class="mpm-cancel-btn" onclick={() => { thoughtsOpen = false; thoughtsText = watched?.thoughts ?? ""; }}>
								Cancel
							</button>
						</div>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<style lang="scss">
	.mpm-backdrop {
		position: fixed;
		inset: 0;
		z-index: 99998;
		background: rgba(0, 0, 0, 0.7);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-end;
		justify-content: center;
	}

	.mpm-modal {
		width: 100%;
		max-height: 85dvh;
		background: $bg-color;
		border-radius: 16px 16px 0 0;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		animation: mpm-slide-up 200ms ease-out;
	}

	@keyframes mpm-slide-up {
		from {
			transform: translateY(100%);
		}
		to {
			transform: translateY(0);
		}
	}

	.mpm-header {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 16px 16px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
		flex-shrink: 0;
	}

	.mpm-poster {
		width: 56px;
		border-radius: 6px;
		aspect-ratio: 2 / 3;
		object-fit: cover;
		flex-shrink: 0;
	}

	.mpm-title-area {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.mpm-title {
		font-size: 17px;
		font-weight: 600;
		color: $text-color;
		margin: 0;
		line-height: 1.3;
		word-wrap: break-word;

		time {
			font-size: 14px;
			font-weight: 400;
			opacity: 0.5;
		}
	}

	.mpm-detail-link {
		font-size: 13px;
		color: $text-color;
		opacity: 0.6;
		padding: 0;
		text-align: left;

		&:active {
			opacity: 1;
		}
	}

	.mpm-close {
		flex-shrink: 0;
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		fill: $text-color;
		opacity: 0.6;
		border-radius: 50%;
		background: rgba(255, 255, 255, 0.08);
		padding: 0;

		&:active {
			opacity: 1;
			background: rgba(255, 255, 255, 0.15);
		}
	}

	.mpm-body {
		overflow-y: auto;
		flex: 1;
		padding-bottom: env(safe-area-inset-bottom, 16px);
	}

	.mpm-section {
		padding: 12px 16px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);

		&:last-child {
			border-bottom: none;
		}
	}

	.mpm-owner-section {
		background: rgba(255, 255, 255, 0.03);
	}

	.mpm-owner-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.mpm-owner-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 10px;
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.08);
		font-size: 13px;
		fill: $text-color;
		text-transform: capitalize;

		:global(svg) {
			width: 14px;
			height: 14px;
			flex-shrink: 0;
		}
	}

	.mpm-owner-thoughts {
		margin-top: 6px;
		font-size: 13px;
		font-style: italic;
		opacity: 0.7;
		line-height: 1.4;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.mpm-label {
		display: block;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.5;
		margin-bottom: 8px;
	}

	.mpm-statuses {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 6px;
	}

	.mpm-status-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 10px;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.06);
		border: 2px solid transparent;
		cursor: pointer;
		color: $text-color;
		fill: $text-color;
		font-size: 13px;
		transition:
			background 150ms,
			border-color 150ms;

		:global(svg) {
			width: 18px;
			height: 18px;
			flex-shrink: 0;
		}

		span {
			text-transform: capitalize;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}

		&:active {
			background: rgba(255, 255, 255, 0.15);
		}

		&.active {
			background: rgba(255, 255, 255, 0.15);
			border-color: rgba(255, 255, 255, 0.3);
		}

		&.delete {
			color: rgba(244, 67, 54, 0.8);
			fill: rgba(244, 67, 54, 0.8);

			&:active {
				background: rgba(244, 67, 54, 0.15);
			}
		}

		&:disabled {
			opacity: 0.5;
			pointer-events: none;
		}
	}

	.mpm-rating {
		display: flex;
		justify-content: center;

		:global(.wrap) {
			width: 100%;
		}
	}

	.mpm-thoughts-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.06);
		color: $text-color;
		fill: $text-color;
		font-size: 14px;

		:global(svg) {
			width: 18px;
			height: 18px;
			flex-shrink: 0;
		}

		&:active {
			background: rgba(255, 255, 255, 0.12);
		}
	}

	.mpm-thoughts-editor {
		display: flex;
		flex-direction: column;
		gap: 8px;

		textarea {
			width: 100%;
			min-height: 80px;
			max-height: 200px;
			padding: 10px;
			border-radius: 8px;
			border: 1px solid rgba(255, 255, 255, 0.15);
			background: rgba(255, 255, 255, 0.06);
			color: $text-color;
			font-size: 14px;
			resize: none;
			outline: none;
			font-family: inherit;

			&:focus {
				border-color: rgba(255, 255, 255, 0.3);
			}
		}

		.mpm-thoughts-actions {
			display: flex;
			gap: 8px;
			justify-content: flex-end;
		}
	}

	.mpm-save-btn {
		padding: 8px 16px;
		font-size: 13px;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.12);
		border: 1px solid rgba(255, 255, 255, 0.2);
		color: $text-color;
		cursor: pointer;

		&:active:not(:disabled) {
			background: rgba(255, 255, 255, 0.2);
		}

		&:disabled {
			opacity: 0.4;
		}
	}

	.mpm-cancel-btn {
		padding: 8px 16px;
		font-size: 13px;
		border-radius: 8px;
		background: none;
		border: 1px solid rgba(255, 255, 255, 0.1);
		color: $text-color;
		opacity: 0.6;

		&:active {
			opacity: 1;
		}
	}
</style>
