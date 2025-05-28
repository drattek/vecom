package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class EntriesDto<T> {
    private List<T> entries;
    private Pagination pagination;

    public EntriesDto() {}

    public EntriesDto(List<T> entries, Pagination pagination) {
        this.entries = entries;
        this.pagination = pagination;
    }

    public List<T> getEntries() {
        return entries;
    }

    public void setEntries(List<T> entries) {
        this.entries = entries;
    }

    public Pagination getPagination() {
        return pagination;
    }

    public void setPagination(Pagination pagination) {
        this.pagination = pagination;
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    public static class Pagination {
        private Integer offset;
        private Integer limit;
        @JsonProperty("total_pages")
        private Integer totalPages;
        @JsonProperty("current_page")
        private Integer currentPage;
        @JsonProperty("next_page")
        private Integer nextPage;
        @JsonProperty("previous_page")
        private Integer previousPage;
        @JsonProperty("total_items")
        private Integer totalItems;
        @JsonProperty("scroll_id")
        private String scrollId;

        public Pagination() {}

        public Pagination(Integer offset, Integer limit, Integer totalPages, Integer currentPage, Integer nextPage, Integer previousPage, Integer totalItems, String scrollId) {
            this.offset = offset;
            this.limit = limit;
            this.totalPages = totalPages;
            this.currentPage = currentPage;
            this.nextPage = nextPage;
            this.previousPage = previousPage;
            this.totalItems = totalItems;
            this.scrollId = scrollId;
        }

        public Integer getOffset() {
            return offset;
        }

        public void setOffset(Integer offset) {
            this.offset = offset;
        }

        public Integer getLimit() {
            return limit;
        }

        public void setLimit(Integer limit) {
            this.limit = limit;
        }

        public Integer getTotalPages() {
            return totalPages;
        }

        public void setTotalPages(Integer totalPages) {
            this.totalPages = totalPages;
        }

        public Integer getCurrentPage() {
            return currentPage;
        }

        public void setCurrentPage(Integer currentPage) {
            this.currentPage = currentPage;
        }

        public Integer getNextPage() {
            return nextPage;
        }

        public void setNextPage(Integer nextPage) {
            this.nextPage = nextPage;
        }

        public Integer getPreviousPage() {
            return previousPage;
        }

        public void setPreviousPage(Integer previousPage) {
            this.previousPage = previousPage;
        }

        public Integer getTotalItems() {
            return totalItems;
        }

        public void setTotalItems(Integer totalItems) {
            this.totalItems = totalItems;
        }

        public String getScrollId() {
            return scrollId;
        }

        public void setScrollId(String scrollId) {
            this.scrollId = scrollId;
        }
    }
}
