<script lang="ts">
	import tooltip from "@/lib/actions/tooltip";
	import DropDown from "@/lib/DropDown.svelte";
	import axios from "axios";
	import {
		DiscoverFilter,
		SearchType,
		type DiscoverFilterOption,
		type DropDownItem,
		type RecommendSourceUser,
	} from "@/types";
	import { onMount } from "svelte";

	interface Props {
		active: string | undefined;
		discoverType: SearchType | undefined;
		onChange: () => void;
	}

	let { active = $bindable(), discoverType, onChange }: Props = $props();

	let recommendSources: RecommendSourceUser[] = $state([]);

	const dropDownOptions: { [x in DiscoverFilterOption]: DropDownItem } = {
		trending: {
			id: DiscoverFilter.trending,
			value: "Trending",
		},
		popular: {
			id: DiscoverFilter.popular,
			value: "Popular",
		},
		upcoming: {
			id: DiscoverFilter.upcoming,
			value: "Upcoming",
		},
		intheatres: {
			id: DiscoverFilter.inTheatres,
			value: "In Theatres",
		},
		streaming: {
			id: DiscoverFilter.streaming,
			value: "Streaming",
		},
		recommended: {
			id: DiscoverFilter.recommended,
			value: "Recommended",
		},
	};

	let options = $derived.by(() => {
		let o: DropDownItem[] = [dropDownOptions.trending];
		const supportsRecommended =
			discoverType === SearchType.multi ||
			discoverType === SearchType.movie ||
			discoverType === SearchType.show;

		switch (discoverType) {
			case SearchType.multi:
				break;
			case SearchType.movie:
				o.push(
					dropDownOptions.popular,
					dropDownOptions.upcoming,
					dropDownOptions.intheatres,
				);
				break;
			case SearchType.show:
				o.push(dropDownOptions.popular, dropDownOptions.upcoming);
				break;
			case SearchType.person:
				o.push(dropDownOptions.popular);
				break;
			case SearchType.game:
				o.push(dropDownOptions.upcoming);
				break;
			case SearchType.manga:
				break;
		}

		if (supportsRecommended) {
			o.push(dropDownOptions.recommended);
			for (const src of recommendSources) {
				o.push({
					id: `recommended:${src.id}`,
					value: `Recommended by ${src.username}`,
				});
			}
		}

		return o;
	});
	let onMultiDiscover = false;

	onMount(async () => {
		try {
			const resp = await axios.get<RecommendSourceUser[]>("/discover/recommend-sources");
			recommendSources = resp.data ?? [];
		} catch (e) {
			console.warn("FilterDropDown: Failed to fetch recommend sources", e);
		}
	});
</script>

<div
	use:tooltip={{
		text: "Must select a type first.",
		pos: "left",
		condition: onMultiDiscover,
	}}
>
	<DropDown
		placeholder="Trending"
		{options}
		isDropDownItem={true}
		bind:active
		{onChange}
		disabled={onMultiDiscover}
	/>
</div>
