<script lang="ts">
	import {
		type Media,
		type Watched,
		type WatchedStatus,
		MediaTypeE,
		type SupportedMedia,
	} from "@/types";
	import { goto } from "$app/navigation";
	import { updateWatched, removeWatched, baseURL } from "../util/api";
	import { watchedStatuses, toUnderstandableStatus } from "../util/helpers";
	import { toShowableRating } from "../rating/helpers";
	import Icon from "../Icon.svelte";
	import PosterContextMenu from "./PosterContextMenu.svelte";

	interface Props {
		items: Media[];
	}

	let { items = $bindable() }: Props = $props();

	type SortKey = "name" | "rating" | "status" | "type" | "added" | "year";
	type SortDir = "asc" | "desc";

	let sortKey: SortKey = $state("name");
	let sortDir: SortDir = $state("asc");
	let searchQuery = $state("");
	let ctxMenu: { x: number; y: number; index: number } | undefined = $state();

	function getMeta(media: Media) {
		let id: number | undefined;
		let type: SupportedMedia;
		switch (media.type) {
			case MediaTypeE.tmdbMovie:
				id = media.ids.tmdb; type = "movie"; break;
			case MediaTypeE.tmdbShow:
				id = media.ids.tmdb; type = "tv"; break;
			case MediaTypeE.igdbGame:
				id = media.ids.igdb; type = "game"; break;
			default:
				return undefined;
		}
		return { id, type };
	}

	function typeLabel(media: Media): string {
		switch (media.type) {
			case MediaTypeE.tmdbMovie: return "Movie";
			case MediaTypeE.tmdbShow: return "Show";
			case MediaTypeE.igdbGame: return "Game";
			default: return "";
		}
	}

	function getYear(media: Media): string {
		return media.releaseDate ? new Date(media.releaseDate).getFullYear().toString() : "—";
	}

	const filtered = $derived.by(() => {
		let list = items.filter(m => m && m.name);
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase();
			list = list.filter(m => m.name!.toLowerCase().includes(q));
		}
		list.sort((a, b) => {
			let cmp = 0;
			switch (sortKey) {
				case "name":
					cmp = (a.name ?? "").localeCompare(b.name ?? ""); break;
				case "rating":
					cmp = (a.watched?.rating ?? 0) - (b.watched?.rating ?? 0); break;
				case "status":
					cmp = (a.watched?.status ?? "").localeCompare(b.watched?.status ?? ""); break;
				case "type":
					cmp = typeLabel(a).localeCompare(typeLabel(b)); break;
				case "added":
					cmp = (a.watched?.createdAt ?? "").localeCompare(b.watched?.createdAt ?? ""); break;
				case "year":
					cmp = getYear(a).localeCompare(getYear(b)); break;
			}
			return sortDir === "asc" ? cmp : -cmp;
		});
		return list;
	});

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === "asc" ? "desc" : "asc";
		} else {
			sortKey = key;
			sortDir = "asc";
		}
	}

	function handleRowClick(media: Media) {
		const meta = getMeta(media);
		if (meta?.id) goto(`/${meta.type}/${meta.id}`);
	}

	function handleRowContext(e: MouseEvent, index: number) {
		e.preventDefault();
		ctxMenu = { x: e.clientX, y: e.clientY, index };
	}

	function handleWatchedUpdate(index: number, w: Watched | undefined) {
		// Find the item in the original items array and update it
		const media = filtered[index];
		const origIndex = items.indexOf(media);
		if (origIndex >= 0) {
			items[origIndex].watched = w;
		}
	}
</script>

<div class="list-view">
	<div class="list-search">
		<Icon i="search" wh={16} />
		<input type="text" placeholder="Search in list..." bind:value={searchQuery} />
	</div>
	<div class="list-table-wrap">
		<table class="list-table">
			<thead>
				<tr>
					<th class="col-name" onclick={() => toggleSort("name")}>
						Name {sortKey === "name" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
					<th class="col-type" onclick={() => toggleSort("type")}>
						Type {sortKey === "type" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
					<th class="col-year" onclick={() => toggleSort("year")}>
						Year {sortKey === "year" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
					<th class="col-status" onclick={() => toggleSort("status")}>
						Status {sortKey === "status" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
					<th class="col-rating" onclick={() => toggleSort("rating")}>
						Rating {sortKey === "rating" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
					<th class="col-added" onclick={() => toggleSort("added")}>
						Added {sortKey === "added" ? (sortDir === "asc" ? "▲" : "▼") : ""}
					</th>
				</tr>
			</thead>
			<tbody>
				{#each filtered as media, i (media.name + "-" + i)}
					<tr
						class="list-row"
						onclick={() => handleRowClick(media)}
						oncontextmenu={(e) => handleRowContext(e, i)}
					>
						<td class="col-name">
							<span class="media-name">{media.name}</span>
							<span class="mobile-meta">
								{typeLabel(media)}{#if getYear(media) !== "—"} · {getYear(media)}{/if}{#if media.watched?.createdAt} · {new Date(media.watched.createdAt).toLocaleDateString()}{/if}
							</span>
						</td>
						<td class="col-type">{typeLabel(media)}</td>
						<td class="col-year">{getYear(media)}</td>
						<td class="col-status">
							{#if media.watched?.status}
								<span class="status-badge status-{media.watched.status.toLowerCase()}">
									<Icon i={watchedStatuses[media.watched.status]} wh={14} />
									{toUnderstandableStatus(media.watched.status, media.type === MediaTypeE.igdbGame)}
								</span>
							{:else}
								<span class="status-badge status-none">—</span>
							{/if}
						</td>
						<td class="col-rating">
							{#if media.watched?.rating}
								<span class="rating-val">{toShowableRating(media.watched.rating)}</span>
							{:else}
								—
							{/if}
						</td>
						<td class="col-added">
							{#if media.watched?.createdAt}
								{new Date(media.watched.createdAt).toLocaleDateString()}
							{:else}
								—
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>

{#if ctxMenu}
	{@const media = filtered[ctxMenu.index]}
	{@const meta = getMeta(media)}
	{#if meta}
		<PosterContextMenu
			x={ctxMenu.x}
			y={ctxMenu.y}
			watched={media.watched}
			contentId={meta.id ?? 0}
			contentType={meta.type}
			mediaName={media.name ?? ""}
			onClose={() => { ctxMenu = undefined; }}
			onWatchedUpdate={(w) => {
				handleWatchedUpdate(ctxMenu!.index, w);
				ctxMenu = undefined;
			}}
		/>
	{/if}
{/if}

<style lang="scss">
	.list-view {
		width: 100%;
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 16px;
	}

	.list-search {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 12px;
		padding: 8px 12px;
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.06);
		border: 1px solid rgba(255, 255, 255, 0.1);
		fill: $text-color;

		:global(svg) {
			flex-shrink: 0;
			opacity: 0.5;
		}

		input {
			flex: 1;
			background: none;
			border: none;
			outline: none;
			color: $text-color;
			font-size: 13px;
			font-family: inherit;

			&::placeholder {
				color: rgba(255, 255, 255, 0.35);
			}
		}
	}

	.list-table-wrap {
		overflow-x: auto;
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.08);
	}

	.list-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;

		thead {
			background: rgba(255, 255, 255, 0.04);

			th {
				padding: 10px 12px;
				text-align: left;
				font-weight: 600;
				font-size: 11px;
				text-transform: uppercase;
				letter-spacing: 0.5px;
				color: $text-color;
				opacity: 0.6;
				cursor: pointer;
				user-select: none;
				white-space: nowrap;
				border-bottom: 1px solid rgba(255, 255, 255, 0.08);

				&:hover {
					opacity: 1;
				}
			}
		}

		tbody {
			.list-row {
				cursor: pointer;
				transition: background 120ms;
				border-bottom: 1px solid rgba(255, 255, 255, 0.04);

				&:hover {
					background: rgba(255, 255, 255, 0.06);
				}

				td {
					padding: 8px 12px;
					color: $text-color;
					white-space: nowrap;
				}
			}
		}
	}

	.col-name {
		min-width: 200px;

		.media-name {
			font-weight: 500;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
			display: block;
			max-width: 400px;
		}

		.mobile-meta {
			display: none;
		}
	}

	.col-type {
		min-width: 60px;
	}

	.col-year, .col-rating, .col-added {
		min-width: 70px;
	}

	.col-status {
		min-width: 100px;
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 12px;
		text-transform: capitalize;
		fill: $text-color;

		:global(svg) {
			width: 14px;
			height: 14px;
		}

		&.status-planned { background: rgba(96, 165, 250, 0.15); color: rgb(96, 165, 250); fill: rgb(96, 165, 250); }
		&.status-watching { background: rgba(251, 191, 36, 0.15); color: rgb(251, 191, 36); fill: rgb(251, 191, 36); }
		&.status-finished { background: rgba(74, 222, 128, 0.15); color: rgb(74, 222, 128); fill: rgb(74, 222, 128); }
		&.status-hold { background: rgba(251, 146, 60, 0.15); color: rgb(251, 146, 60); fill: rgb(251, 146, 60); }
		&.status-dropped { background: rgba(248, 113, 113, 0.15); color: rgb(248, 113, 113); fill: rgb(248, 113, 113); }
		&.status-none { opacity: 0.4; }
	}

	.rating-val {
		font-weight: 500;
	}

	@media screen and (max-width: 600px) {
		.list-view {
			padding: 0;
		}

		.list-table-wrap {
			border: none;
			border-radius: 0;
			overflow-x: visible;
		}

		.list-table {
			thead {
				display: none;
			}

			tbody {
				display: flex;
				flex-direction: column;

				.list-row {
					display: grid;
					grid-template-columns: 1fr auto;
					grid-template-rows: auto auto;
					gap: 2px 12px;
					padding: 10px 14px;
					background: none;
					border: none;
					border-bottom: 1px solid rgba(255, 255, 255, 0.08);
					border-radius: 0;

					&:nth-child(odd) {
						background: rgba(255, 255, 255, 0.03);
					}

					&:hover {
						background: rgba(255, 255, 255, 0.06);
					}

					td {
						padding: 0;
						white-space: normal;
						background-color: transparent !important;
					}

					.col-name {
						grid-column: 1 / -1;
						grid-row: 1;
						min-width: 0;

						.media-name {
							max-width: none;
							font-size: 14px;
							font-weight: 600;
							white-space: nowrap;
							overflow: hidden;
							text-overflow: ellipsis;
						}

						.mobile-meta {
							display: block;
							font-size: 11px;
							color: $text-color-accent;
							opacity: 0.5;
							margin-top: 1px;
						}
					}

					.col-status {
						grid-column: 1;
						grid-row: 2;
						margin-top: 4px;
					}

					.col-rating {
						grid-column: 2;
						grid-row: 2;
						text-align: right;
						display: flex;
						align-items: center;
						justify-content: flex-end;
						margin-top: 4px;

						.rating-val {
							font-size: 13px;
						}
					}

					.col-type, .col-year, .col-added {
						display: none;
					}
				}
			}
		}

		.status-badge {
			font-size: 11px;
			padding: 2px 6px;
		}
	}
</style>
