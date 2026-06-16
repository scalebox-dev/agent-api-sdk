export interface PageParams {
  limit?: number;
  page_token?: string;
}

export interface PageResult<T> {
  data: T[];
  has_more: boolean;
  next_page_token?: string;
}

export class Page<TItem, TParams extends PageParams> implements AsyncIterable<TItem> {
  constructor(
    private readonly fetchPage: (params: TParams) => Promise<PageResult<TItem>>,
    public readonly params: TParams,
    public readonly data: TItem[],
    public readonly hasMore: boolean,
    public readonly nextPageToken?: string,
  ) {}

  [Symbol.asyncIterator](): AsyncIterator<TItem> {
    return this.iterateAll()[Symbol.asyncIterator]();
  }

  async getNextPage(): Promise<Page<TItem, TParams> | null> {
    if (!this.hasMore || !this.nextPageToken) {
      return null;
    }
    const nextParams = { ...this.params, page_token: this.nextPageToken } as TParams;
    const result = await this.fetchPage(nextParams);
    return new Page(this.fetchPage, nextParams, result.data, result.has_more, result.next_page_token);
  }

  async *iterateAll(): AsyncGenerator<TItem, void, undefined> {
    let page: Page<TItem, TParams> | null = this;
    while (page) {
      for (const item of page.data) {
        yield item;
      }
      page = await page.getNextPage();
    }
  }
}

export async function collectPage<TItem, TParams extends PageParams>(
  fetchPage: (params: TParams) => Promise<PageResult<TItem>>,
  params: TParams,
): Promise<Page<TItem, TParams>> {
  const result = await fetchPage(params);
  return new Page(fetchPage, params, result.data, result.has_more, result.next_page_token);
}
