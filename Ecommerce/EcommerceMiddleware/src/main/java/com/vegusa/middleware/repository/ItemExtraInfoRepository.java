package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ItemExtraInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface ItemExtraInfoRepository  extends JpaRepository<ItemExtraInfo, Long> {
    @Query(value = "select * from itemextrainfo where InterfaceId = ?1 and DataAreaId = ?2", nativeQuery = true)
    ItemExtraInfo[] getItemExtraInfo(String interfaceId, String dataAreaId);
}
