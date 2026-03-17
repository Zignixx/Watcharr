<script lang="ts">
	import {
		type WatchedStatus,
		type Watched,
		type SupportedMedia,
	} from "@/types";
	import { updateWatched, removeWatched } from "../util/api";
	import { notify } from "../util/notify";
	import Icon from "../Icon.svelte";
	import { watchedStatuses, toUnderstandableStatus } from "../util/helpers";
	import Rating from "../rating/Rating.svelte";
	import { toShowableRating } from "../rating/helpers";
	import { onMount, onDestroy } from "svelte";

	interface Props {
		x: number;
		y: number;
		watched: Watched | undefined;
		contentId: number;
		contentType: SupportedMedia;
		mediaName: string;
		onClose: () => void;
		onWatchedUpdate: (w: Watched | undefined) => void;
		ownerWatched?: Watched;
		ownerName?: string;
	}

	let {
		x,
		y,
		watched,
		contentId,
		contentType,
		mediaName,
		onClose,
		onWatchedUpdate,
		ownerWatched,
		ownerName,
	}: Props = $props();

	let menuEl: HTMLDivElement | undefined = $state();
	let thoughtsOpen = $state(!!watched?.thoughts);
	let thoughtsText = $state(watched?.thoughts ?? "");
	let textarea: HTMLTextAreaElement | undefined = $state();
	let saving = $state(false);

	// Position the menu within viewport bounds
	let menuX = $state(x);
	let menuY = $state(y);

	onMount(() => {
		if (menuEl) {
			const rect = menuEl.getBoundingClientRect();
			const vw = window.innerWidth;
			const vh = window.innerHeight;
			if (x + rect.width > vw) menuX = vw - rect.width - 10;
			if (y + rect.height > vh) menuY = vh - rect.height - 10;
			if (menuX < 0) menuX = 10;
			if (menuY < 0) menuY = 10;
		}
		document.addEventListener("mousedown", handleOutsideClick);
	});

	onDestroy(() => {
		document.removeEventListener("mousedown", handleOutsideClick);
	});

	function handleOutsideClick(e: MouseEvent) {
		if (menuEl && !menuEl.contains(e.target as Node)) {
			onClose();
		}
	}

	async function handleStatusClick(status: WatchedStatus) {
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

	async function handleDelete() {
		if (!watched) return;
		saving = true;
		const removed = await removeWatched(watched.id);
		if (removed) {
			onWatchedUpdate(undefined);
		}
		saving = false;
		onClose();
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
			// If item is not on the list yet, add it first
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

	$effect(() => {
		if (textarea) resizeTextarea();
	});
</script>

<div class="ctx-backdrop" role="presentation"></div>
<div
	class="ctx-menu"
	bind:this={menuEl}
	style="left: {menuX}px; top: {menuY}px;"
	role="menu"
>
	<div class="ctx-header">
		<button class="ctx-close" onclick={onClose}>
			<Icon i="close" wh={16} />
		</button>
		<span class="ctx-title">{mediaName}</span>
		<div class="ctx-close-spacer"></div>
	</div>

	<!-- Owner info (read-only) -->
	{#if ownerWatched}
		<div class="ctx-owner-section">
			<span class="ctx-label">{ownerName ? `${ownerName}'s Info` : "Owner's Info"}</span>
			<div class="ctx-owner-row">
				{#if ownerWatched.status}
					<span class="ctx-owner-badge">
						<Icon i={watchedStatuses[ownerWatched.status]} wh={14} />
						{toUnderstandableStatus(ownerWatched.status, contentType === "game")}
					</span>
				{/if}
				{#if ownerWatched.rating}
					<span class="ctx-owner-badge">★ {toShowableRating(ownerWatched.rating)}</span>
				{/if}
			</div>
			{#if ownerWatched.thoughts}
				<div class="ctx-owner-thoughts">"{ownerWatched.thoughts}"</div>
			{/if}
		</div>
	{/if}

	<!-- Status Section -->
	<div class="ctx-section">
		<span class="ctx-label">Status</span>
		<div class="ctx-statuses">
			{#each Object.entries(watchedStatuses) as [statusName, icon]}
				<button
					class="ctx-status-btn"
					class:active={watched?.status === statusName}
					onclick={() => handleStatusClick(statusName as WatchedStatus)}
					disabled={saving}
					title={toUnderstandableStatus(statusName as WatchedStatus, contentType === "game")}
				>
					<Icon i={icon} wh={18} />
					<span>{toUnderstandableStatus(statusName as WatchedStatus, contentType === "game")}</span>
				</button>
			{/each}
			{#if watched}
				<button
					class="ctx-status-btn delete"
					onclick={handleDelete}
					disabled={saving}
					title="Remove from list"
				>
					<Icon i="trash" wh={18} />
					<span>Remove</span>
				</button>
			{/if}
		</div>
	</div>

	<!-- Rating Section -->
	<div class="ctx-section">
		<span class="ctx-label">Rating</span>
		<div class="ctx-rating">
			<Rating rating={watched?.rating} onChange={handleRatingChange} />
		</div>
	</div>

	<!-- Thoughts Section -->
	<div class="ctx-section">
		{#if !thoughtsOpen}
			<button
				class="ctx-thoughts-btn"
				onclick={() => { thoughtsOpen = true; }}
			>
				<Icon i="pencil" wh={16} />
				<span>
					{#if watched?.thoughts}
						Edit thoughts
					{:else}
						Set thoughts on {mediaName}
					{/if}
				</span>
			</button>
		{:else}
			<div class="ctx-thoughts-editor">
				<textarea
					bind:this={textarea}
					bind:value={thoughtsText}
					oninput={resizeTextarea}
					placeholder="Your thoughts on {mediaName}"
					rows="3"
				></textarea>
				<div class="ctx-thoughts-actions">
					<button class="ctx-save-btn" onclick={handleThoughtsSave} disabled={saving}>
						Save
					</button>
					<button class="ctx-cancel-btn" onclick={() => { thoughtsOpen = false; thoughtsText = watched?.thoughts ?? ""; }}>
						Cancel
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>

<style lang="scss">
	.ctx-backdrop {
		position: fixed;
		inset: 0;
		z-index: 99998;
		background: transparent;
	}

	.ctx-menu {
		position: fixed;
		z-index: 99999;
		background: $bg-color;
		border: 1px solid rgba(255, 255, 255, 0.12);
		border-radius: 8px;
		min-width: 260px;
		max-width: 420px;
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
		overflow: hidden;
		font-size: 13px;
		color: $text-color;
	}

	.ctx-header {
		display: flex;
		align-items: center;
		padding: 10px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);

		.ctx-title {
			font-weight: 600;
			font-size: 13px;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
			flex: 1;
			text-align: center;
			min-width: 0;
		}

		.ctx-close, .ctx-close-spacer {
			width: 24px;
			height: 24px;
			flex-shrink: 0;
		}

		.ctx-close {
			display: flex;
			align-items: center;
			justify-content: center;
			background: none;
			border: none;
			cursor: pointer;
			fill: $text-color;
			opacity: 0.6;
			padding: 2px;
			border-radius: 4px;
			transition: opacity 150ms;

			:global(svg) {
				width: 16px;
				height: 16px;
			}

			&:hover {
				opacity: 1;
			}
		}
	}

	.ctx-section {
		padding: 8px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);

		&:last-child {
			border-bottom: none;
		}
	}

	.ctx-owner-section {
		padding: 8px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
		background: rgba(255, 255, 255, 0.03);
	}

	.ctx-owner-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.ctx-owner-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 8px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.08);
		font-size: 12px;
		fill: $text-color;
		text-transform: capitalize;

		:global(svg) {
			width: 14px;
			height: 14px;
			flex-shrink: 0;
		}
	}

	.ctx-owner-thoughts {
		margin-top: 6px;
		font-size: 12px;
		font-style: italic;
		opacity: 0.7;
		line-height: 1.4;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.ctx-label {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.5;
		margin-bottom: 6px;

		em {
			font-style: normal;
			opacity: 0.8;
			margin-left: auto;
			text-transform: none;
			letter-spacing: 0;
			font-size: 12px;
		}
	}

	.ctx-statuses {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 4px;
	}

	.ctx-status-btn {
		display: flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border-radius: 5px;
		background: rgba(255, 255, 255, 0.06);
		border: 1px solid transparent;
		cursor: pointer;
		color: $text-color;
		fill: $text-color;
		font-size: 12px;
		transition:
			background 150ms,
			border-color 150ms;

		:global(svg) {
			width: 16px;
			height: 16px;
			flex-shrink: 0;
		}

		&:hover {
			background: rgba(255, 255, 255, 0.12);
		}

		&.active {
			background: rgba(255, 255, 255, 0.15);
			border-color: rgba(255, 255, 255, 0.3);
		}

		&.delete {
			color: rgba(244, 67, 54, 0.8);
			fill: rgba(244, 67, 54, 0.8);

			&:hover {
				background: rgba(244, 67, 54, 0.15);
			}
		}

		span {
			text-transform: capitalize;
		}

		&:disabled {
			opacity: 0.5;
			pointer-events: none;
		}
	}

	.ctx-rating {
		display: flex;
		justify-content: center;
		overflow: hidden;

		:global(.wrap) {
			width: 100%;
		}
	}

	.ctx-save-btn {
		padding: 4px 10px;
		font-size: 11px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.12);
		border: 1px solid rgba(255, 255, 255, 0.2);
		color: $text-color;
		cursor: pointer;
		transition:
			background 150ms,
			opacity 150ms;
		white-space: nowrap;

		&:hover:not(:disabled) {
			background: rgba(255, 255, 255, 0.2);
		}

		&:disabled {
			opacity: 0.4;
			cursor: default;
		}
	}

	.ctx-cancel-btn {
		padding: 4px 10px;
		font-size: 11px;
		border-radius: 4px;
		background: none;
		border: 1px solid rgba(255, 255, 255, 0.1);
		color: $text-color;
		cursor: pointer;
		opacity: 0.6;
		transition: opacity 150ms;

		&:hover {
			opacity: 1;
		}
	}

	.ctx-thoughts-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border-radius: 5px;
		background: rgba(255, 255, 255, 0.06);
		border: none;
		cursor: pointer;
		color: $text-color;
		fill: $text-color;
		font-size: 12px;
		transition: background 150ms;

		:global(svg) {
			width: 16px;
			height: 16px;
			flex-shrink: 0;
		}

		&:hover {
			background: rgba(255, 255, 255, 0.12);
		}

		span {
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}
	}

	.ctx-thoughts-editor {
		display: flex;
		flex-direction: column;
		gap: 6px;

		textarea {
			width: 100%;
			min-height: 60px;
			max-height: 150px;
			padding: 8px;
			border-radius: 5px;
			border: 1px solid rgba(255, 255, 255, 0.15);
			background: rgba(255, 255, 255, 0.06);
			color: $text-color;
			font-size: 12px;
			resize: none;
			outline: none;
			font-family: inherit;

			&:focus {
				border-color: rgba(255, 255, 255, 0.3);
			}
		}

		.ctx-thoughts-actions {
			display: flex;
			gap: 6px;
			justify-content: flex-end;
		}
	}
</style>
