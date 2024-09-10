package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Channel;
import com.vegusa.middleware.entity.ChannelId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ChannelRepository extends JpaRepository<Channel, ChannelId> {
    @Query(value = "select * from Channel where Name = ?1 and CurrencyCode = ?2 and DataAreaId = ?3", nativeQuery = true)
    Channel getChannel(String name, String currencyCode, String dataAreaId);
}
