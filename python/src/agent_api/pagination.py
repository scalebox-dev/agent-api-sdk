from __future__ import annotations

from collections.abc import AsyncIterator, Callable, Iterator, Mapping
from typing import Any, Generic, TypeVar

TItem = TypeVar("TItem")
TParams = TypeVar("TParams", bound=Mapping[str, Any])


class PageResult(Generic[TItem]):
    def __init__(self, *, data: list[TItem], has_more: bool, next_page_token: str | None = None) -> None:
        self.data = data
        self.has_more = has_more
        self.next_page_token = next_page_token


class Page(Generic[TItem, TParams]):
    def __init__(
        self,
        fetch_page: Callable[[TParams], PageResult[TItem]],
        params: TParams,
        result: PageResult[TItem],
    ) -> None:
        self._fetch_page = fetch_page
        self.params = params
        self.data = result.data
        self.has_more = result.has_more
        self.next_page_token = result.next_page_token

    def __iter__(self) -> Iterator[TItem]:
        return self.iter_all()

    def get_next_page(self) -> Page[TItem, TParams] | None:
        if not self.has_more or not self.next_page_token:
            return None
        next_params = dict(self.params)
        next_params["page_token"] = self.next_page_token
        result = self._fetch_page(next_params)  # type: ignore[arg-type]
        return Page(self._fetch_page, next_params, result)  # type: ignore[arg-type]

    def iter_all(self) -> Iterator[TItem]:
        page: Page[TItem, TParams] | None = self
        while page is not None:
            yield from page.data
            page = page.get_next_page()


class AsyncPage(Generic[TItem, TParams]):
    def __init__(
        self,
        fetch_page: Callable[[TParams], Any],
        params: TParams,
        result: PageResult[TItem],
    ) -> None:
        self._fetch_page = fetch_page
        self.params = params
        self.data = result.data
        self.has_more = result.has_more
        self.next_page_token = result.next_page_token

    def __aiter__(self) -> AsyncIterator[TItem]:
        return self.iter_all()

    async def get_next_page(self) -> AsyncPage[TItem, TParams] | None:
        if not self.has_more or not self.next_page_token:
            return None
        next_params = dict(self.params)
        next_params["page_token"] = self.next_page_token
        result = await self._fetch_page(next_params)  # type: ignore[misc]
        return AsyncPage(self._fetch_page, next_params, result)  # type: ignore[arg-type]

    async def iter_all(self) -> AsyncIterator[TItem]:
        page: AsyncPage[TItem, TParams] | None = self
        while page is not None:
            for item in page.data:
                yield item
            page = await page.get_next_page()
